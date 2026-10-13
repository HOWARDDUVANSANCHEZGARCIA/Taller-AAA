package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

// newTestRouter arma la API con una ruta protegida de prueba (/protegida)
// que responde con el usuario identificado por el middleware.
func newTestRouter(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo, err := user.NewFileRepository(filepath.Join(t.TempDir(), "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.NewService(repo, auth.NewTokenManager([]byte("secreto-de-pruebas"), time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	RegisterAuthRoutes(r, svc)
	r.GET("/protegida", Authenticate(svc), func(c *gin.Context) {
		u, _ := CurrentUser(c)
		c.JSON(http.StatusOK, gin.H{"id": u.ID, "rol": u.Rol})
	})
	return r, svc
}

func do(r http.Handler, method, path, body, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const registerBody = `{"nombre":"Ana Pérez","email":"ana@correo.com","telefono":"3001234567","password":"clave-segura-1"}`

func TestRegisterThenLogin(t *testing.T) {
	r, _ := newTestRouter(t)

	w := do(r, http.MethodPost, "/auth/register", registerBody, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("registro: status %d, cuerpo %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Error("la respuesta de registro no debe incluir contraseña ni hash")
	}

	w = do(r, http.MethodPost, "/auth/login", `{"email":"ana@correo.com","password":"clave-segura-1"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login: status %d, cuerpo %s", w.Code, w.Body)
	}
	var res loginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res.Token == "" || res.Tipo != "Bearer" {
		t.Errorf("respuesta de login inesperada: %s", w.Body)
	}
}

func TestRegisterIgnoresRoleSentByClient(t *testing.T) {
	r, _ := newTestRouter(t)
	body := `{"nombre":"Mallory","email":"m@correo.com","telefono":"3001234567","password":"clave-segura-1","rol":"administrador"}`

	w := do(r, http.MethodPost, "/auth/register", body, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, cuerpo %s", w.Code, w.Body)
	}
	var res userResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Rol != user.RoleVendedor {
		t.Errorf("rol = %q; el cliente no puede elegir su rol, quería %q", res.Rol, user.RoleVendedor)
	}
}

func TestRegisterErrors(t *testing.T) {
	r, _ := newTestRouter(t)
	do(r, http.MethodPost, "/auth/register", registerBody, "")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"correo repetido", registerBody, http.StatusConflict},
		{"JSON inválido", `{no-es-json`, http.StatusBadRequest},
		{"datos inválidos", `{"nombre":"","email":"x","telefono":"1","password":"1"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if w := do(r, http.MethodPost, "/auth/register", tc.body, ""); w.Code != tc.want {
			t.Errorf("%s: status %d; quería %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestLoginWrongCredentialsIs401(t *testing.T) {
	r, _ := newTestRouter(t)
	do(r, http.MethodPost, "/auth/register", registerBody, "")

	for name, body := range map[string]string{
		"clave incorrecta":   `{"email":"ana@correo.com","password":"equivocada"}`,
		"correo inexistente": `{"email":"nadie@correo.com","password":"clave-segura-1"}`,
	} {
		if w := do(r, http.MethodPost, "/auth/login", body, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d; quería 401", name, w.Code)
		}
	}
}

func TestProtectedRouteWithoutCredentials(t *testing.T) {
	r, _ := newTestRouter(t)

	for name, header := range map[string]string{
		"sin encabezado":  "",
		"esquema erróneo": "Basic abc",
		"token vacío":     "Bearer ",
		"token falso":     "Bearer token-falso",
	} {
		w := do(r, http.MethodGet, "/protegida", "", header)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d; quería 401", name, w.Code)
		}
		if w.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: falta el encabezado WWW-Authenticate", name)
		}
	}
}

func TestProtectedRouteWithValidToken(t *testing.T) {
	r, _ := newTestRouter(t)
	do(r, http.MethodPost, "/auth/register", registerBody, "")
	w := do(r, http.MethodPost, "/auth/login", `{"email":"ana@correo.com","password":"clave-segura-1"}`, "")
	var login loginResponse
	_ = json.Unmarshal(w.Body.Bytes(), &login)

	w = do(r, http.MethodGet, "/protegida", "", "Bearer "+login.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, cuerpo %s", w.Code, w.Body)
	}
	var got struct {
		Rol user.Role `json:"rol"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Rol != user.RoleVendedor {
		t.Errorf("el rol que ve la ruta protegida es %q; quería %q", got.Rol, user.RoleVendedor)
	}
}

func TestCurrentUserOutsideProtectedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := CurrentUser(c); ok {
		t.Error("CurrentUser no debe devolver usuario si no pasó por Authenticate")
	}
}
