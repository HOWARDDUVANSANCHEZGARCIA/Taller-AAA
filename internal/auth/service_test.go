package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

func newTestService(t *testing.T) (*Service, *user.FileRepository) {
	t.Helper()
	repo, err := user.NewFileRepository(filepath.Join(t.TempDir(), "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(repo, NewTokenManager([]byte("secreto-de-pruebas"), time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

func validInput() RegisterInput {
	return RegisterInput{Nombre: "Ana Pérez", Email: "Ana@Correo.com", Telefono: "+57 300 123 4567", Password: "clave-segura-1"}
}

func TestRegisterCreatesVendedor(t *testing.T) {
	svc, _ := newTestService(t)

	u, err := svc.Register(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if u.Rol != user.RoleVendedor {
		t.Errorf("Rol = %q; quería %q", u.Rol, user.RoleVendedor)
	}
	if u.Email != "ana@correo.com" {
		t.Errorf("Email = %q; debe normalizarse a minúsculas", u.Email)
	}
	if u.PasswordHash == "" || u.PasswordHash == "clave-segura-1" {
		t.Error("la contraseña debe guardarse como hash")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Register(validInput()); err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.Email = "ANA@correo.com"
	if _, err := svc.Register(in); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("err = %v; quería ErrEmailTaken", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	svc, _ := newTestService(t)
	mutate := map[string]func(*RegisterInput){
		"nombre vacío":        func(in *RegisterInput) { in.Nombre = "  " },
		"correo inválido":     func(in *RegisterInput) { in.Email = "no-es-correo" },
		"correo con nombre":   func(in *RegisterInput) { in.Email = "Ana <ana@correo.com>" },
		"teléfono corto":      func(in *RegisterInput) { in.Telefono = "123" },
		"teléfono con letras": func(in *RegisterInput) { in.Telefono = "300-ABC-4567" },
		"clave corta":         func(in *RegisterInput) { in.Password = "corta" },
	}
	for name, f := range mutate {
		in := validInput()
		f(&in)
		var verr *ValidationError
		if _, err := svc.Register(in); !errors.As(err, &verr) {
			t.Errorf("%s: err = %v; quería ValidationError", name, err)
		}
	}
}

func TestLogin(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Register(validInput()); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Login("ana@correo.com", "clave-segura-1"); err != nil {
		t.Errorf("login válido falló: %v", err)
	}
	if _, err := svc.Login(" ANA@correo.com ", "clave-segura-1"); err != nil {
		t.Errorf("login con mayúsculas/espacios falló: %v", err)
	}
	if _, err := svc.Login("ana@correo.com", "equivocada"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("clave incorrecta: err = %v; quería ErrInvalidCredentials", err)
	}
	if _, err := svc.Login("nadie@correo.com", "clave-segura-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("correo inexistente: err = %v; quería ErrInvalidCredentials", err)
	}
}

func TestAuthenticate(t *testing.T) {
	svc, _ := newTestService(t)
	registered, _ := svc.Register(validInput())
	session, err := svc.Login("ana@correo.com", "clave-segura-1")
	if err != nil {
		t.Fatal(err)
	}

	u, err := svc.Authenticate(session.Token)
	if err != nil || u.ID != registered.ID {
		t.Fatalf("Authenticate = %+v, %v; quería el usuario registrado", u, err)
	}
	if _, err := svc.Authenticate("token-falso"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("token falso: err = %v; quería ErrUnauthenticated", err)
	}
	if _, err := svc.Authenticate(""); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("sin token: err = %v; quería ErrUnauthenticated", err)
	}
}

func TestAuthenticateTokenOfUnknownUser(t *testing.T) {
	svc, _ := newTestService(t)
	token, _ := svc.tokens.Issue("usuario-que-no-existe")
	if _, err := svc.Authenticate(token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v; quería ErrUnauthenticated", err)
	}
}

func TestSeedAdmin(t *testing.T) {
	svc, repo := newTestService(t)
	in := RegisterInput{Nombre: "Admin", Email: "admin@automarket.com", Telefono: "3001234567", Password: "admin-clave-1"}

	if err := svc.SeedAdmin(in); err != nil {
		t.Fatal(err)
	}
	if err := svc.SeedAdmin(in); err != nil {
		t.Fatalf("sembrar dos veces no debe fallar: %v", err)
	}
	u, err := repo.FindByEmail("admin@automarket.com")
	if err != nil || u.Rol != user.RoleAdministrador {
		t.Errorf("admin = %+v, %v; quería rol administrador", u, err)
	}
}
