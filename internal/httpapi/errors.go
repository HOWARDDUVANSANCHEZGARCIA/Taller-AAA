// Package httpapi contiene la capa HTTP (Gin): rutas, handlers y middleware
// de la API JSON y de las páginas web.
package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// errForbidden: el solicitante está autenticado pero su rol no le permite la operación.
var errForbidden = errors.New("no tiene permiso para realizar esta operación")

// errorStatus traduce un error del sistema al código HTTP y al mensaje que
// ve el cliente. Es el único lugar donde se hace esta traducción, tanto para
// la API como para las páginas web. Los errores inesperados no se exponen.
func errorStatus(err error) (int, string) {
	var verr *apperr.ValidationError
	switch {
	case errors.As(err, &verr):
		return http.StatusBadRequest, verr.Msg
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		return http.StatusUnauthorized, err.Error()
	case errors.Is(err, errForbidden), errors.Is(err, vehicle.ErrNotOwner):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, vehicle.ErrNotFound), errors.Is(err, user.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, user.ErrEmailTaken), errors.Is(err, vehicle.ErrAlreadySold),
		errors.Is(err, vehicle.ErrNotPending), errors.Is(err, admin.ErrSelfDelete):
		return http.StatusConflict, err.Error()
	default:
		log.Printf("error interno: %v", err)
		return http.StatusInternalServerError, "error interno del servidor"
	}
}

// abortWithError corta la solicitud con una respuesta de error JSON uniforme.
func abortWithError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// respondServiceError responde un error de la API con la forma {"error": "..."}.
func respondServiceError(c *gin.Context, err error) {
	status, msg := errorStatus(err)
	if status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", "Bearer")
	}
	abortWithError(c, status, msg)
}
