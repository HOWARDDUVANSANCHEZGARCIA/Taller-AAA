package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// WebHandler sirve las páginas HTML. Usa los mismos servicios que la API, así
// que las reglas (validaciones, estados, dueño del vehículo) son las mismas.
type WebHandler struct {
	auth     *auth.Service
	vehicles *vehicle.Service
	admin    *admin.Service
	pages    webPages
}

// RegisterWebRoutes expone las páginas del sitio. El catálogo, ingresar y
// registrarse son públicos; publicar y "mis vehículos" exigen rol vendedor, y
// el panel exige rol administrador. El panel usa /panel porque /admin es de la API.
func RegisterWebRoutes(r gin.IRouter, s Services) {
	h := &WebHandler{auth: s.Auth, vehicles: s.Vehicles, admin: s.Admin, pages: loadWebPages()}

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
	seller.GET("/mis-vehiculos/:id/fotos", h.photos)
	seller.POST("/mis-vehiculos/:id/fotos", h.addPhotos)
	seller.POST("/mis-vehiculos/:id/fotos/:nombre/quitar", h.removePhoto)

	panel := web.Group("/panel", h.pages.requireRole(user.RoleAdministrador))
	panel.GET("", h.adminPanel)
	panel.POST("/vehiculos/:id/aprobar", h.adminApprove)
	panel.POST("/vehiculos/:id/eliminar", h.adminDeletePublication)
	panel.POST("/usuarios/:id/eliminar", h.adminDeleteUser)
}

// notices son los avisos que se muestran después de redirigir (?aviso=...).
var notices = map[string]string{
	"registrado": "Tu cuenta quedó creada. Ingresa con tu correo y contraseña.",
	"publicado":  "Publicación enviada. Aparecerá en el catálogo cuando un administrador la apruebe.",
	"vendido":    "Venta reportada. El vehículo salió del catálogo.",
	"aprobada":   "Publicación aprobada. Ya aparece en el catálogo.",
	"eliminada":  "Publicación eliminada.",
	"eliminado":  "Usuario eliminado junto con sus publicaciones.",
	"agregadas":  "Fotos agregadas.",
	"quitada":    "Foto quitada.",
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
	c.Set(currentUserKey, session.User) // el accounting registra quién ingresó
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

func (h *WebHandler) publishForm(c *gin.Context) {
	h.pages.render(c, http.StatusOK, "publicar", page{Data: publishForm{}})
}

func (h *WebHandler) publish(c *gin.Context) {
	form, in, err := readPublishForm(c)
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

// photosData es lo que muestra la página para cambiar las fotos.
type photosData struct {
	Vehicle  vehicle.Vehicle
	Editable bool // sigue pendiente
	Max      int
}

func (h *WebHandler) photos(c *gin.Context) {
	h.renderPhotos(c, http.StatusOK, "")
}

func (h *WebHandler) renderPhotos(c *gin.Context, status int, errMsg string) {
	seller, _ := CurrentUser(c)
	v, err := h.vehicles.Own(seller.ID, c.Param("id"))
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	h.pages.render(c, status, "fotos", page{
		Error:  errMsg,
		Notice: notices[c.Query("aviso")],
		Data:   photosData{Vehicle: v, Editable: v.Estado == vehicle.StatusPendiente, Max: vehicle.MaxImages},
	})
}

func (h *WebHandler) addPhotos(c *gin.Context) {
	images, err := readImages(c)
	if err == nil {
		seller, _ := CurrentUser(c)
		_, err = h.vehicles.AddImages(seller.ID, c.Param("id"), images)
	}
	if err != nil {
		status, msg := errorStatus(err)
		h.renderPhotos(c, status, msg)
		return
	}
	c.Redirect(http.StatusSeeOther, "/mis-vehiculos/"+c.Param("id")+"/fotos?aviso=agregadas")
}

func (h *WebHandler) removePhoto(c *gin.Context) {
	seller, _ := CurrentUser(c)
	if _, err := h.vehicles.RemoveImage(seller.ID, c.Param("id"), c.Param("nombre")); err != nil {
		status, msg := errorStatus(err)
		h.renderPhotos(c, status, msg)
		return
	}
	c.Redirect(http.StatusSeeOther, "/mis-vehiculos/"+c.Param("id")+"/fotos?aviso=quitada")
}

// adminPanelData es lo que muestra el panel del administrador.
type adminPanelData struct {
	Publications []admin.Publication
	Users        []user.User
	SelfID       string
}

func (h *WebHandler) adminPanel(c *gin.Context) {
	pubs, err := h.admin.Publications()
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	users, err := h.admin.Users()
	if err != nil {
		h.pages.renderError(c, err)
		return
	}
	current, _ := CurrentUser(c)
	h.pages.render(c, http.StatusOK, "panel", page{
		Notice: notices[c.Query("aviso")],
		Data:   adminPanelData{Publications: pubs, Users: users, SelfID: current.ID},
	})
}

func (h *WebHandler) adminApprove(c *gin.Context) {
	if _, err := h.admin.Approve(c.Param("id")); err != nil {
		h.pages.renderError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/panel?aviso=aprobada")
}

func (h *WebHandler) adminDeletePublication(c *gin.Context) {
	if err := h.admin.DeletePublication(c.Param("id")); err != nil {
		h.pages.renderError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/panel?aviso=eliminada")
}

func (h *WebHandler) adminDeleteUser(c *gin.Context) {
	current, _ := CurrentUser(c)
	if err := h.admin.DeleteUser(current.ID, c.Param("id")); err != nil {
		h.pages.renderError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/panel?aviso=eliminado")
}
