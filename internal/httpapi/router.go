package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// Services agrupa los servicios que necesita la capa HTTP.
type Services struct {
	Auth     *auth.Service
	Vehicles *vehicle.Service
}

// NewRouter arma todas las rutas de la aplicación. Lo usan main y las
// pruebas, para que ambos ejecuten exactamente la misma configuración.
func NewRouter(s Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	// No hay proxy delante del servidor: c.ClientIP() usa la IP real de la
	// conexión y no el encabezado X-Forwarded-For, que el cliente puede
	// falsificar (importa para el registro de accounting). Con nil no falla.
	_ = r.SetTrustedProxies(nil)

	RegisterAuthRoutes(r, s.Auth)
	RegisterVehicleRoutes(r, s.Vehicles, s.Auth)
	RegisterWebRoutes(r, s)
	return r
}
