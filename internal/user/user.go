// Package user define el modelo de usuario del sistema y el contrato de
// persistencia que usan los demás componentes (autenticación, autorización,
// administración).
package user

import (
	"errors"
	"time"
)

// Role es el rol de un usuario autenticado. El visitante no es un rol
// almacenado: es toda persona que no presenta credenciales válidas.
type Role string

const (
	RoleVendedor      Role = "vendedor"
	RoleAdministrador Role = "administrador"
)

// User es un usuario registrado. El rol solo lo asigna el servidor.
type User struct {
	ID           string    `json:"id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	Telefono     string    `json:"telefono"`
	PasswordHash string    `json:"password_hash"`
	Rol          Role      `json:"rol"`
	CreadoEn     time.Time `json:"creado_en"`
}

var (
	ErrEmailTaken = errors.New("el correo ya está registrado")
	ErrNotFound   = errors.New("usuario no encontrado")
)

// Repository es el contrato de persistencia de usuarios.
type Repository interface {
	// Create guarda un usuario nuevo. Devuelve ErrEmailTaken si el correo ya existe.
	Create(u User) error
	// FindByEmail devuelve ErrNotFound si no existe.
	FindByEmail(email string) (User, error)
	// FindByID devuelve ErrNotFound si no existe.
	FindByID(id string) (User, error)
}
