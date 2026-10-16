package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// Las plantillas y el CSS van dentro del ejecutable: el servidor funciona
// sin importar desde qué carpeta se lance.
//
//go:embed web
var webFS embed.FS

// sessionCookie guarda el mismo token que entrega POST /auth/login.
//   - HttpOnly: el JavaScript de la página no puede leerlo.
//   - SameSite=Strict: el navegador no lo envía en solicitudes que vienen de
//     otros sitios, lo que impide que otra página haga acciones a nombre del
//     usuario (CSRF).
//   - Sin Secure: el enunciado pide HTTP sin TLS en esta entrega.
const sessionCookie = "sesion"

// page es lo que reciben todas las plantillas.
type page struct {
	User   *user.User // nil si es un visitante
	Error  string
	Notice string
	Data   any
}

// webPages tiene una plantilla por página, cada una combinada con el layout.
type webPages map[string]*template.Template

func loadWebPages() webPages {
	funcs := template.FuncMap{
		"pesos":  pesos,
		"miles":  miles,
		"estado": func(s vehicle.Status) string { return vehicleStatusLabel[s] },
		"tel":    telURL,
		"imagen": imageURL,
		"fotos":  photoCount,
		"suma":   func(a, b int) int { return a + b },
		// anterior y siguiente dan la vuelta: de la primera a la última y al revés.
		"anterior":  func(i, n int) int { return (i - 1 + n) % n },
		"siguiente": func(i, n int) int { return (i + 1) % n },
	}
	pages := webPages{}
	for _, name := range []string{"catalogo", "vehiculo", "ingresar", "registro", "publicar", "mis-vehiculos", "fotos", "panel", "error"} {
		pages[name] = template.Must(template.New(name).Funcs(funcs).
			ParseFS(webFS, "web/templates/layout.html", "web/templates/"+name+".html"))
	}
	return pages
}

func (p webPages) render(c *gin.Context, status int, name string, data page) {
	if u, ok := CurrentUser(c); ok {
		data.User = &u
	}
	c.Render(status, render.HTML{Template: p[name], Name: "layout", Data: data})
}

// renderError muestra la página de error con el código y mensaje de errorStatus.
func (p webPages) renderError(c *gin.Context, err error) {
	status, msg := errorStatus(err)
	p.render(c, status, "error", page{Error: msg, Data: status})
	c.Abort()
}

// webIdentify reconoce al usuario por la cookie de sesión. A diferencia de
// Authenticate, no bloquea a nadie: el catálogo también es para visitantes.
func webIdentify(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookie)
		if err != nil {
			c.Next()
			return
		}
		u, err := svc.Authenticate(token)
		if err != nil {
			clearSession(c) // token vencido o de un usuario eliminado
			c.Next()
			return
		}
		c.Set(currentUserKey, u)
		c.Next()
	}
}

// requireRole envía al visitante a ingresar y muestra 403 si el rol no
// alcanza. Es el equivalente de Authenticate + RequireRole para las páginas.
func (p webPages) requireRole(roles ...user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			c.Redirect(http.StatusSeeOther, "/ingresar")
			c.Abort()
			return
		}
		if !u.HasRole(roles...) {
			p.renderError(c, errForbidden)
			return
		}
		c.Next()
	}
}

func setSession(c *gin.Context, s auth.Session) {
	maxAge := int(time.Until(s.ExpiresAt).Seconds())
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookie, s.Token, maxAge, "/", "", false, true)
}

func clearSession(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookie, "", -1, "/", "", false, true)
}

// pesos da formato colombiano: 78000000 -> "$ 78.000.000".
func pesos(n int64) string { return "$ " + groupThousands(n) }

// miles da formato a enteros: 60000 -> "60.000".
func miles(n int) string { return groupThousands(int64(n)) }

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return sign + s
}

// telURL arma el enlace "tel:" con solo dígitos y "+". html/template no
// acepta el esquema tel: por defecto; como aquí se filtra cada carácter,
// es seguro marcarlo como URL confiable.
func telURL(phone string) template.URL {
	var b strings.Builder
	for _, r := range phone {
		if (r >= '0' && r <= '9') || r == '+' {
			b.WriteRune(r)
		}
	}
	return template.URL("tel:" + b.String())
}

// photoCount: 1 -> "1 foto", 3 -> "3 fotos".
func photoCount(n int) string {
	if n == 1 {
		return "1 foto"
	}
	return strconv.Itoa(n) + " fotos"
}

// vehicleStatusLabel es el texto que ve el vendedor para cada estado.
var vehicleStatusLabel = map[vehicle.Status]string{
	vehicle.StatusPendiente: "Pendiente de aprobación",
	vehicle.StatusPublicada: "Publicado",
	vehicle.StatusVendida:   "Vendido",
}
