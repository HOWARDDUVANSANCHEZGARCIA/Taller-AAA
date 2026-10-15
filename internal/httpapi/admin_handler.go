package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

type AdminHandler struct {
	svc *admin.Service
}

// RegisterAdminRoutes expone la moderación (HU-ADM-01). Todas las rutas
// exigen un usuario autenticado con rol administrador.
func RegisterAdminRoutes(r gin.IRouter, svc *admin.Service, authSvc *auth.Service) {
	h := &AdminHandler{svc: svc}

	g := r.Group("/admin", Authenticate(authSvc), RequireRole(user.RoleAdministrador))
	g.GET("/vehiculos", h.publications)
	g.POST("/vehiculos/:id/aprobar", h.approve)
	g.DELETE("/vehiculos/:id", h.deletePublication)
	g.GET("/usuarios", h.users)
	g.DELETE("/usuarios/:id", h.deleteUser)
}

type publicationResponse struct {
	vehicleResponse
	Vendedor string `json:"vendedor"`
}

type adminUserResponse struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	Email    string    `json:"email"`
	Telefono string    `json:"telefono"`
	Rol      user.Role `json:"rol"`
	CreadoEn time.Time `json:"creado_en"`
}

func (h *AdminHandler) publications(c *gin.Context) {
	pubs, err := h.svc.Publications()
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res := make([]publicationResponse, 0, len(pubs))
	for _, p := range pubs {
		res = append(res, publicationResponse{vehicleResponse: toVehicleResponse(p.Vehicle), Vendedor: p.Vendedor})
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) approve(c *gin.Context) {
	v, err := h.svc.Approve(c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleResponse(v))
}

func (h *AdminHandler) deletePublication(c *gin.Context) {
	if err := h.svc.DeletePublication(c.Param("id")); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) users(c *gin.Context) {
	users, err := h.svc.Users()
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res := make([]adminUserResponse, 0, len(users))
	for _, u := range users {
		res = append(res, adminUserResponse{
			ID: u.ID, Nombre: u.Nombre, Email: u.Email, Telefono: u.Telefono, Rol: u.Rol, CreadoEn: u.CreadoEn,
		})
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) deleteUser(c *gin.Context) {
	current, _ := CurrentUser(c)
	if err := h.svc.DeleteUser(current.ID, c.Param("id")); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
