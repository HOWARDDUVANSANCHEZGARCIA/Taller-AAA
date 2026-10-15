package httpapi

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/accounting"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

const currentUserKey = "auth.user"

// Authenticate protege las rutas que lo usan. Exige el encabezado
//
//	Authorization: Bearer <token>
//
// Si el token es válido, deja al usuario (con el rol que tiene en el
// servidor) disponible con CurrentUser. Si no, responde 401 y corta.
func Authenticate(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			respondServiceError(c, auth.ErrUnauthenticated)
			return
		}
		u, err := svc.Authenticate(token)
		if err != nil {
			respondServiceError(c, err)
			return
		}
		c.Set(currentUserKey, u)
		c.Next()
	}
}

// CurrentUser devuelve el usuario autenticado de la solicitud. El segundo
// valor es false si la ruta no pasó por Authenticate.
func CurrentUser(c *gin.Context) (user.User, bool) {
	v, exists := c.Get(currentUserKey)
	if !exists {
		return user.User{}, false
	}
	u, ok := v.(user.User)
	return u, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

// RequireRole deja pasar solo a usuarios autenticados con alguno de los roles
// indicados; si no, responde 403. Va después de Authenticate:
//
//	r.POST("/vehiculos", Authenticate(svc), RequireRole(user.RoleVendedor), h.publish)
func RequireRole(roles ...user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			respondServiceError(c, auth.ErrUnauthenticated)
			return
		}
		if !u.HasRole(roles...) {
			respondServiceError(c, errForbidden)
			return
		}
		c.Next()
	}
}

// Accounting registra cada solicitud después de procesarla, cuando ya se
// conocen el usuario (si Authenticate o el login lo identificaron) y el
// resultado. La hoja de estilos no se registra: no es una operación.
//
// Si el registro falla, la respuesta ya se envió; el error queda en el log
// del servidor para que el operador lo revise.
func Accounting(l *accounting.Log) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			return
		}
		actor, rol := "anonimo", "visitante"
		if u, ok := CurrentUser(c); ok {
			actor, rol = u.ID, string(u.Rol)
		}
		route := c.FullPath()
		if route == "" {
			route = "(ruta inexistente)"
		}
		err := l.Record(accounting.Entry{
			Operacion: c.Request.Method + " " + route,
			Recurso:   c.Request.URL.Path,
			Actor:     actor,
			Rol:       rol,
			Resultado: c.Writer.Status(),
			IP:        c.ClientIP(),
		})
		if err != nil {
			log.Printf("accounting: no se pudo registrar %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		}
	}
}
