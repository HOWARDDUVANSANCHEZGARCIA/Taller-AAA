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
	r.GET("/vehiculos/:id/imagenes/:nombre", OptionalAuthenticate(authSvc), h.image)

	seller := r.Group("", Authenticate(authSvc), RequireRole(user.RoleVendedor))
	seller.POST("/vehiculos", h.publish)
	seller.POST("/vehiculos/:id/venta", h.reportSale)
	seller.POST("/vehiculos/:id/imagenes", h.addImages)
	seller.DELETE("/vehiculos/:id/imagenes/:nombre", h.removeImage)
	seller.GET("/vendedor/vehiculos", h.mine)
}

type vehicleResponse struct {
	ID          string         `json:"id"`
	Marca       string         `json:"marca"`
	Modelo      string         `json:"modelo"`
	Anio        int            `json:"anio"`
	Precio      int64          `json:"precio"`
	Kilometraje int            `json:"kilometraje"`
	Estado      vehicle.Status `json:"estado"`
	Imagenes    []string       `json:"imagenes"` // URLs; la primera es la portada
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

// publish recibe multipart/form-data (los datos y las fotos). El vendedor y
// el estado no se leen del formulario: los fija el servidor.
func (h *VehicleHandler) publish(c *gin.Context) {
	_, in, err := readPublishForm(c)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	seller, _ := CurrentUser(c)
	v, err := h.svc.Publish(seller.ID, in)
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

// addImages agrega fotos (multipart, campo "imagenes") a una publicación
// propia mientras sigue pendiente.
func (h *VehicleHandler) addImages(c *gin.Context) {
	images, err := readImages(c)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	seller, _ := CurrentUser(c)
	v, err := h.svc.AddImages(seller.ID, c.Param("id"), images)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleResponse(v))
}

// removeImage quita una foto de una publicación propia mientras sigue pendiente.
func (h *VehicleHandler) removeImage(c *gin.Context) {
	seller, _ := CurrentUser(c)
	v, err := h.svc.RemoveImage(seller.ID, c.Param("id"), c.Param("nombre"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleResponse(v))
}

// image entrega una foto si el solicitante puede verla (ver
// vehicle.Service.ImagePath). nosniff impide que el navegador la interprete
// como algo distinto de una imagen.
func (h *VehicleHandler) image(c *gin.Context) {
	var viewer *user.User
	if u, ok := CurrentUser(c); ok {
		viewer = &u
	}
	path, err := h.svc.ImagePath(viewer, c.Param("id"), c.Param("nombre"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=300")
	c.File(path)
}

func toVehicleResponse(v vehicle.Vehicle) vehicleResponse {
	urls := make([]string, 0, len(v.Imagenes))
	for _, name := range v.Imagenes {
		urls = append(urls, imageURL(v.ID, name))
	}
	return vehicleResponse{
		ID: v.ID, Marca: v.Marca, Modelo: v.Modelo, Anio: v.Anio,
		Precio: v.Precio, Kilometraje: v.Kilometraje, Estado: v.Estado,
		Imagenes: urls, CreadoEn: v.CreadoEn,
	}
}

func toListingResponse(l vehicle.Listing) listingResponse {
	return listingResponse{
		vehicleResponse: toVehicleResponse(l.Vehicle),
		Vendedor:        contactResponse{Nombre: l.Vendedor.Nombre, Email: l.Vendedor.Email, Telefono: l.Vendedor.Telefono},
	}
}
