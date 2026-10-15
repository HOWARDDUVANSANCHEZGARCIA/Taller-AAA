package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

type VehicleHandler struct {
	svc *vehicle.Service
}

// RegisterVehicleRoutes expone el catálogo (público, rol visitante) y las
// operaciones del vendedor (autenticado y con rol vendedor).
func RegisterVehicleRoutes(r gin.IRouter, vehicles *vehicle.Service, authSvc *auth.Service) {
	h := &VehicleHandler{svc: vehicles}

	r.GET("/vehiculos", h.catalog)
	r.GET("/vehiculos/:id", h.catalogItem)

	seller := r.Group("", Authenticate(authSvc), RequireRole(user.RoleVendedor))
	seller.POST("/vehiculos", h.publish)
	seller.POST("/vehiculos/:id/venta", h.reportSale)
	seller.GET("/vendedor/vehiculos", h.mine)
}

// publishRequest no tiene vendedor ni estado: los fija el servidor.
type publishRequest struct {
	Marca       string `json:"marca"`
	Modelo      string `json:"modelo"`
	Anio        int    `json:"anio"`
	Precio      int64  `json:"precio"`
	Kilometraje int    `json:"kilometraje"`
}

type vehicleResponse struct {
	ID          string         `json:"id"`
	Marca       string         `json:"marca"`
	Modelo      string         `json:"modelo"`
	Anio        int            `json:"anio"`
	Precio      int64          `json:"precio"`
	Kilometraje int            `json:"kilometraje"`
	Estado      vehicle.Status `json:"estado"`
	CreadoEn    time.Time      `json:"creado_en"`
}

type contactResponse struct {
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Telefono string `json:"telefono"`
}

type listingResponse struct {
	vehicleResponse
	Vendedor contactResponse `json:"vendedor"`
}

func (h *VehicleHandler) catalog(c *gin.Context) {
	listings, err := h.svc.Catalog()
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res := make([]listingResponse, 0, len(listings))
	for _, l := range listings {
		res = append(res, toListingResponse(l))
	}
	c.JSON(http.StatusOK, res)
}

func (h *VehicleHandler) catalogItem(c *gin.Context) {
	l, err := h.svc.CatalogItem(c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toListingResponse(l))
}

func (h *VehicleHandler) publish(c *gin.Context) {
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "el cuerpo debe ser un JSON válido")
		return
	}
	seller, _ := CurrentUser(c)
	v, err := h.svc.Publish(seller.ID, vehicle.PublishInput{
		Marca:       req.Marca,
		Modelo:      req.Modelo,
		Anio:        req.Anio,
		Precio:      req.Precio,
		Kilometraje: req.Kilometraje,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toVehicleResponse(v))
}

func (h *VehicleHandler) reportSale(c *gin.Context) {
	seller, _ := CurrentUser(c)
	v, err := h.svc.ReportSale(seller.ID, c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleResponse(v))
}

func (h *VehicleHandler) mine(c *gin.Context) {
	seller, _ := CurrentUser(c)
	own, err := h.svc.BySeller(seller.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res := make([]vehicleResponse, 0, len(own))
	for _, v := range own {
		res = append(res, toVehicleResponse(v))
	}
	c.JSON(http.StatusOK, res)
}

func toVehicleResponse(v vehicle.Vehicle) vehicleResponse {
	return vehicleResponse{
		ID: v.ID, Marca: v.Marca, Modelo: v.Modelo, Anio: v.Anio,
		Precio: v.Precio, Kilometraje: v.Kilometraje, Estado: v.Estado, CreadoEn: v.CreadoEn,
	}
}

func toListingResponse(l vehicle.Listing) listingResponse {
	return listingResponse{
		vehicleResponse: toVehicleResponse(l.Vehicle),
		Vendedor:        contactResponse{Nombre: l.Vendedor.Nombre, Email: l.Vendedor.Email, Telefono: l.Vendedor.Telefono},
	}
}
