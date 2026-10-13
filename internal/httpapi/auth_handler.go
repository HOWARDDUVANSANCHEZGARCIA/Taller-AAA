package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

type AuthHandler struct {
	svc *auth.Service
}

// RegisterAuthRoutes expone el registro de vendedores y el inicio de sesión.
// Ambas rutas son públicas: no pasan por el middleware Authenticate.
func RegisterAuthRoutes(r gin.IRoutes, svc *auth.Service) {
	h := &AuthHandler{svc: svc}
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
}

// registerRequest no tiene campo de rol a propósito: el rol lo fija el servidor.
type registerRequest struct {
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Telefono string `json:"telefono"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	Email    string    `json:"email"`
	Telefono string    `json:"telefono"`
	Rol      user.Role `json:"rol"`
}

type loginResponse struct {
	Token    string    `json:"token"`
	Tipo     string    `json:"tipo"`
	ExpiraEn time.Time `json:"expira_en"`
}

func (h *AuthHandler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "el cuerpo debe ser un JSON válido")
		return
	}
	u, err := h.svc.Register(auth.RegisterInput{
		Nombre:   req.Nombre,
		Email:    req.Email,
		Telefono: req.Telefono,
		Password: req.Password,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, userResponse{
		ID: u.ID, Nombre: u.Nombre, Email: u.Email, Telefono: u.Telefono, Rol: u.Rol,
	})
}

func (h *AuthHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "el cuerpo debe ser un JSON válido")
		return
	}
	session, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: session.Token, Tipo: "Bearer", ExpiraEn: session.ExpiresAt})
}
