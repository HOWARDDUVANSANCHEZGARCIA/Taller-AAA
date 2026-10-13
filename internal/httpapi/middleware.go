package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"

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
