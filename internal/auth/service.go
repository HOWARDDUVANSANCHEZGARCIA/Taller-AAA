// Package auth implementa la capa de Authentication del marco AAA:
// registro de vendedores, inicio de sesión e identificación del solicitante.
package auth

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/ids"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
	maxNameLen     = 100
	minPhoneDigits = 7
	maxPhoneDigits = 15
)

var (
	// ErrInvalidCredentials es deliberadamente genérico: no revela si el
	// correo existe o si lo incorrecto fue la contraseña.
	ErrInvalidCredentials = errors.New("credenciales inválidas")
	// ErrUnauthenticated indica que el solicitante no pudo ser identificado.
	ErrUnauthenticated = errors.New("no autenticado")
)

// ValidationError describe un dato de entrada inválido. Es el mismo tipo
// que usan los demás componentes (apperr), para manejar los errores igual.
type ValidationError = apperr.ValidationError

// dummyHash se compara cuando el correo no existe, para que el login tarde
// lo mismo en ambos casos y no se pueda saber por el tiempo de respuesta
// qué correos están registrados.
var dummyHash = mustHash("clave-de-relleno-para-igualar-tiempos")

type RegisterInput struct {
	Nombre   string
	Email    string
	Telefono string
	Password string
}

// Session es el resultado de un inicio de sesión exitoso.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      user.User // quién inició sesión (lo usa el registro de accounting)
}

type Service struct {
	users  user.Repository
	tokens *TokenManager
}

func NewService(users user.Repository, tokens *TokenManager) *Service {
	return &Service{users: users, tokens: tokens}
}

// Register registra un vendedor. El rol siempre lo fija el servidor.
func (s *Service) Register(in RegisterInput) (user.User, error) {
	return s.create(in, user.RoleVendedor)
}

// EnsureAdmin crea el administrador si todavía no existe. El registro público
// solo crea vendedores, así que esta es la única forma de tener un
// administrador: la llama main al arrancar con datos de variables de entorno.
// El teléfono no es obligatorio porque el administrador no se contacta en el catálogo.
func (s *Service) EnsureAdmin(nombre, email, password string) error {
	existing, err := s.users.FindByEmail(strings.TrimSpace(email))
	switch {
	case err == nil && existing.Rol == user.RoleAdministrador:
		return nil
	case err == nil:
		return fmt.Errorf("el correo %s ya pertenece a una cuenta que no es de administrador", existing.Email)
	case !errors.Is(err, user.ErrNotFound):
		return err
	}
	_, err = s.create(RegisterInput{Nombre: nombre, Email: email, Password: password}, user.RoleAdministrador)
	return err
}

func (s *Service) create(in RegisterInput, rol user.Role) (user.User, error) {
	in.Nombre = strings.TrimSpace(in.Nombre)
	in.Email = strings.TrimSpace(in.Email)
	in.Telefono = strings.TrimSpace(in.Telefono)
	if err := validate(in, rol); err != nil {
		return user.User{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return user.User{}, err
	}
	id, err := ids.New()
	if err != nil {
		return user.User{}, err
	}
	u := user.User{
		ID:           id,
		Nombre:       in.Nombre,
		Email:        strings.ToLower(in.Email),
		Telefono:     in.Telefono,
		PasswordHash: hash,
		Rol:          rol,
		CreadoEn:     time.Now().UTC(),
	}
	if err := s.users.Create(u); err != nil {
		return user.User{}, err
	}
	return u, nil
}

// Login verifica las credenciales y emite un token.
func (s *Service) Login(email, password string) (Session, error) {
	u, err := s.users.FindByEmail(strings.TrimSpace(email))
	if errors.Is(err, user.ErrNotFound) {
		VerifyPassword(password, dummyHash)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return Session{}, ErrInvalidCredentials
	}

	token, expiresAt := s.tokens.Issue(u.ID)
	return Session{Token: token, ExpiresAt: expiresAt, User: u}, nil
}

// Authenticate identifica al solicitante a partir de un token. El usuario y
// su rol se leen del repositorio, nunca del token ni del cliente. Si el
// usuario fue eliminado, el token deja de servir de inmediato.
func (s *Service) Authenticate(token string) (user.User, error) {
	id, err := s.tokens.Verify(token)
	if err != nil {
		return user.User{}, ErrUnauthenticated
	}
	u, err := s.users.FindByID(id)
	if errors.Is(err, user.ErrNotFound) {
		return user.User{}, ErrUnauthenticated
	}
	if err != nil {
		return user.User{}, err
	}
	return u, nil
}

func validate(in RegisterInput, rol user.Role) error {
	if in.Nombre == "" || len(in.Nombre) > maxNameLen {
		return &ValidationError{Msg: "el nombre es obligatorio y debe tener máximo 100 caracteres"}
	}
	if addr, err := mail.ParseAddress(in.Email); err != nil || addr.Address != in.Email {
		return &ValidationError{Msg: "el correo no es válido"}
	}
	// El teléfono es el contacto público del vendedor en el catálogo.
	if rol == user.RoleVendedor && !validPhone(in.Telefono) {
		return &ValidationError{Msg: "el teléfono debe tener entre 7 y 15 dígitos"}
	}
	if n := len(in.Password); n < minPasswordLen || n > maxPasswordLen {
		return &ValidationError{Msg: "la contraseña debe tener entre 8 y 128 caracteres"}
	}
	return nil
}

// validPhone acepta dígitos con +, espacios, guiones y paréntesis como separadores.
func validPhone(phone string) bool {
	digits := 0
	for _, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= minPhoneDigits && digits <= maxPhoneDigits
}

func mustHash(password string) string {
	hash, err := HashPassword(password)
	if err != nil {
		panic(err)
	}
	return hash
}
