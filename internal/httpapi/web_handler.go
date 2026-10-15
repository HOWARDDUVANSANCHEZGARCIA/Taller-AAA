package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// WebHandler sirve las páginas HTML. Usa los mismos servicios que la API, así
// que las reglas (validaciones, estados, dueño del vehículo) son las mismas.
type WebHandler struct {
	auth     *auth.Service
	vehicles *vehicle.Service
	pages    webPages
}

// RegisterWebRoutes expone las páginas del sitio. El catálogo, ingresar y
// registrarse son públicos; publicar y "mis vehículos" exigen rol vendedor.
func RegisterWebRoutes(r gin.IRouter, s Services) {
	h := &WebHandler{auth: s.Auth, vehicles: s.Vehicles, pages: loadWebPages()}

	r.StaticFileFS("/static/estilos.css", "web/static/estilos.css", http.FS(webFS))

	web := r.Group("", webIdentify(s.Auth))
	web.GET("/", h.catalog)
	web.GET("/catalogo/:id", h.vehicleDetail)
	web.GET("/ingresar", h.loginForm)
	web.POST("/ingresar", h.login)
	web.GET("/registro", h.registerForm)
	web.POST("/registro", h.register)
	web.POST("/salir", h.logout)

	seller := web.Group("", h.pages.requireRole(user.RoleVendedor))
	seller.GET("/publicar", h.publishForm)
	seller.POST("/publicar", h.publish)
	seller.GET("/mis-vehiculos", h.mine)
	seller.POST("/mis-vehiculos/:id/vendido", h.reportSale)
}

// notices son los avisos que se muestran después de redirigir (?aviso=...).
var notices = map[string]string{
	"registrado": "Tu cuenta quedó creada. Ingresa con tu correo y contraseña.",
	"publicado":  "Publicación enviada. Aparecerá en el catálogo cuando un administrador la apruebe.",
	"vendido":    "Venta reportada. El vehículo salió del catálogo.",
}

func (h *WebHandler) catalog(c *gin.Context) {
	listings, err := h.vehicles.Catalog()
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	h.pages.render(c, http.StatusOK, "catalogo", page{Data: listings})
}

func (h *WebHandler) vehicleDetail(c *gin.Context) {
	l, err := h.vehicles.CatalogItem(c.Param("id"))
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	h.pages.render(c, http.StatusOK, "vehiculo", page{Data: l})
}

type loginForm struct{ Email string }

func (h *WebHandler) loginForm(c *gin.Context) {
	h.pages.render(c, http.StatusOK, "ingresar", page{Notice: notices[c.Query("aviso")], Data: loginForm{}})
}

func (h *WebHandler) login(c *gin.Context) {
	form := loginForm{Email: c.PostForm("email")}
	session, err := h.auth.Login(form.Email, c.PostForm("password"))
	if err != nil {
		status, msg := errorStatus(err)
		h.pages.render(c, status, "ingresar", page{Error: msg, Data: form})
		return
	}
	setSession(c, session)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *WebHandler) logout(c *gin.Context) {
	clearSession(c)
	c.Redirect(http.StatusSeeOther, "/")
}

// registerForm guarda lo escrito para devolverlo si hay un error (nunca la contraseña).
type registerForm struct{ Nombre, Email, Telefono string }

func (h *WebHandler) registerForm(c *gin.Context) {
	h.pages.render(c, http.StatusOK, "registro", page{Data: registerForm{}})
}

func (h *WebHandler) register(c *gin.Context) {
	form := registerForm{Nombre: c.PostForm("nombre"), Email: c.PostForm("email"), Telefono: c.PostForm("telefono")}
	_, err := h.auth.Register(auth.RegisterInput{
		Nombre:   form.Nombre,
		Email:    form.Email,
		Telefono: form.Telefono,
		Password: c.PostForm("password"),
	})
	if err != nil {
		status, msg := errorStatus(err)
		h.pages.render(c, status, "registro", page{Error: msg, Data: form})
		return
	}
	c.Redirect(http.StatusSeeOther, "/ingresar?aviso=registrado")
}

// publishForm guarda lo escrito tal cual, para devolverlo si hay un error.
type publishForm struct{ Marca, Modelo, Anio, Precio, Kilometraje string }

func (h *WebHandler) publishForm(c *gin.Context) {
	h.pages.render(c, http.StatusOK, "publicar", page{Data: publishForm{}})
}

func (h *WebHandler) publish(c *gin.Context) {
	form := publishForm{
		Marca:       c.PostForm("marca"),
		Modelo:      c.PostForm("modelo"),
		Anio:        c.PostForm("anio"),
		Precio:      c.PostForm("precio"),
		Kilometraje: c.PostForm("kilometraje"),
	}
	in, err := form.toInput()
	if err == nil {
		seller, _ := CurrentUser(c)
		_, err = h.vehicles.Publish(seller.ID, in)
	}
	if err != nil {
		status, msg := errorStatus(err)
		h.pages.render(c, status, "publicar", page{Error: msg, Data: form})
		return
	}
	c.Redirect(http.StatusSeeOther, "/mis-vehiculos?aviso=publicado")
}

func (h *WebHandler) mine(c *gin.Context) {
	seller, _ := CurrentUser(c)
	own, err := h.vehicles.BySeller(seller.ID)
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	h.pages.render(c, http.StatusOK, "mis-vehiculos", page{Notice: notices[c.Query("aviso")], Data: own})
}

func (h *WebHandler) reportSale(c *gin.Context) {
	seller, _ := CurrentUser(c)
	if _, err := h.vehicles.ReportSale(seller.ID, c.Param("id")); err != nil {
		h.pages.renderError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/mis-vehiculos?aviso=vendido")
}

// toInput convierte los textos del formulario en números. Acepta puntos y
// espacios como separadores de miles ("78.000.000").
func (f publishForm) toInput() (vehicle.PublishInput, error) {
	anio, err := strconv.Atoi(strings.TrimSpace(f.Anio))
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el año debe ser un número, por ejemplo 2019"}
	}
	precio, err := strconv.ParseInt(stripThousands(f.Precio), 10, 64)
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el precio debe ser un número entero de pesos, por ejemplo 78.000.000"}
	}
	km, err := strconv.Atoi(stripThousands(f.Kilometraje))
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el kilometraje debe ser un número entero, por ejemplo 60.000"}
	}
	return vehicle.PublishInput{Marca: f.Marca, Modelo: f.Modelo, Anio: anio, Precio: precio, Kilometraje: km}, nil
}

// stripThousands quita los separadores de miles; cualquier otro carácter se deja
// para que strconv lo rechace.
func stripThousands(s string) string {
	return strings.NewReplacer(".", "", " ", "").Replace(strings.TrimSpace(s))
}
