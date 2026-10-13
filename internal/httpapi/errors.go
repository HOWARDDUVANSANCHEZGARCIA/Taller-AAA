// Package httpapi contiene la capa HTTP (Gin): rutas, handlers y middleware.
package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

// abortWithError corta la solicitud con una respuesta de error JSON uniforme.
func abortWithError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// respondServiceError traduce los errores del servicio a códigos HTTP.
// Los errores inesperados no se exponen al cliente.
func respondServiceError(c *gin.Context, err error) {
	var verr *auth.ValidationError
	switch {
	case errors.As(err, &verr):
		abortWithError(c, http.StatusBadRequest, verr.Msg)
	case errors.Is(err, user.ErrEmailTaken):
		abortWithError(c, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		c.Header("WWW-Authenticate", "Bearer")
		abortWithError(c, http.StatusUnauthorized, err.Error())
	default:
		log.Printf("error interno: %v", err)
		abortWithError(c, http.StatusInternalServerError, "error interno del servidor")
	}
}
