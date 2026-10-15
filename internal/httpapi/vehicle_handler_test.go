package httpapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/accounting"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// testApp es la aplicación completa (NewRouter) con datos en un directorio temporal.
type testApp struct {
	router   *gin.Engine
	users    *user.FileRepository
	vehicles *vehicle.FileRepository
	tokens   *auth.TokenManager
	authSvc  *auth.Service
	logPath  string // registro de accounting
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	users, err := user.NewFileRepository(filepath.Join(dir, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	vehicles, err := vehicle.NewFileRepository(filepath.Join(dir, "vehicles.json"))
	if err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenManager([]byte("secreto-de-pruebas"), time.Hour)
	authSvc := auth.NewService(users, tokens)
	vehicleSvc := vehicle.NewService(vehicles, users)
	logPath := filepath.Join(dir, "accounting.log")
	accLog, err := accounting.Open(logPath, []byte("clave-accounting-pruebas"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accLog.Close() })
	return &testApp{
		router: NewRouter(Services{
			Auth:       authSvc,
			Vehicles:   vehicleSvc,
			Admin:      admin.NewService(users, vehicleSvc),
			Accounting: accLog,
		}),
		logPath:  logPath,
		users:    users,
		vehicles: vehicles,
		tokens:   tokens,
		authSvc:  authSvc,
	}
}

// addUser crea un usuario directamente en el repositorio y devuelve
// el encabezado Authorization con un token válido para él.
func (a *testApp) addUser(t *testing.T, id string, rol user.Role) string {
	t.Helper()
	u := user.User{ID: id, Nombre: "Usuario " + id, Email: id + "@correo.com", Telefono: "3001234567", Rol: rol}
	if err := a.users.Create(u); err != nil {
		t.Fatal(err)
	}
	token, _ := a.tokens.Issue(id)
	return "Bearer " + token
}

// approve simula la aprobación del administrador (HU-ADM-01).
func (a *testApp) approve(t *testing.T, id string) {
	t.Helper()
	if _, err := a.vehicles.Update(id, func(v *vehicle.Vehicle) error { v.Estado = vehicle.StatusPublicada; return nil }); err != nil {
		t.Fatal(err)
	}
}

const vehicleBody = `{"marca":"Renault","modelo":"Koleos","anio":2019,"precio":78000000,"kilometraje":60000}`

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("respuesta no es JSON válido: %s", body)
	}
	return v
}

func TestPublishRequiresAuthenticatedSeller(t *testing.T) {
	app := newTestApp(t)
	admin := app.addUser(t, "admin", user.RoleAdministrador)

	cases := []struct {
		name, auth string
		want       int
	}{
		{"visitante sin token", "", http.StatusUnauthorized},
		{"token falso", "Bearer falso", http.StatusUnauthorized},
		{"administrador", admin, http.StatusForbidden},
	}
	for _, tc := range cases {
		if w := do(app.router, http.MethodPost, "/vehiculos", vehicleBody, tc.auth); w.Code != tc.want {
			t.Errorf("%s: status %d; quería %d", tc.name, w.Code, tc.want)
		}
	}
	if all, _ := app.vehicles.List(); len(all) != 0 {
		t.Errorf("no debió crearse ninguna publicación; hay %d", len(all))
	}
}

func TestSellerPublishesPendingVehicle(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)

	// El cliente intenta fijar estado y vendedor: el servidor los ignora.
	body := `{"marca":"Renault","modelo":"Koleos","anio":2019,"precio":78000000,"kilometraje":60000,"estado":"publicada","vendedor_id":"otro"}`
	w := do(app.router, http.MethodPost, "/vehiculos", body, seller)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, cuerpo %s", w.Code, w.Body)
	}
	res := decode[vehicleResponse](t, w.Body.Bytes())
	if res.Estado != vehicle.StatusPendiente {
		t.Errorf("estado = %q; quería pendiente", res.Estado)
	}
	if stored, _ := app.vehicles.FindByID(res.ID); stored.VendedorID != "ana" {
		t.Errorf("vendedor = %q; quería el del token", stored.VendedorID)
	}

	for name, bad := range map[string]string{
		"JSON inválido":   `{no-json`,
		"precio en texto": `{"marca":"Renault","modelo":"Koleos","anio":2019,"precio":"caro"}`,
		"datos inválidos": `{"marca":"","modelo":"Koleos","anio":2019,"precio":1}`,
	} {
		if w := do(app.router, http.MethodPost, "/vehiculos", bad, seller); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d; quería 400", name, w.Code)
		}
	}
}

func TestVisitorSeesOnlyPublishedVehiclesWithContact(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	pending := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleBody, seller).Body.Bytes())
	published := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleBody, seller).Body.Bytes())
	app.approve(t, published.ID)

	w := do(app.router, http.MethodGet, "/vehiculos", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("catálogo: status %d", w.Code)
	}
	list := decode[[]listingResponse](t, w.Body.Bytes())
	if len(list) != 1 || list[0].ID != published.ID {
		t.Fatalf("catálogo = %+v; quería solo la publicada", list)
	}
	want := contactResponse{Nombre: "Usuario ana", Email: "ana@correo.com", Telefono: "3001234567"}
	if list[0].Vendedor != want {
		t.Errorf("contacto = %+v; quería %+v", list[0].Vendedor, want)
	}

	w = do(app.router, http.MethodGet, "/vehiculos/"+published.ID, "", "")
	if item := decode[listingResponse](t, w.Body.Bytes()); w.Code != http.StatusOK || item.Vendedor != want {
		t.Errorf("detalle: status %d, %+v", w.Code, item)
	}
	for _, id := range []string{pending.ID, "no-existe"} {
		if w := do(app.router, http.MethodGet, "/vehiculos/"+id, "", ""); w.Code != http.StatusNotFound {
			t.Errorf("detalle %s: status %d; quería 404", id, w.Code)
		}
	}
}

func TestEmptyCatalogIsEmptyList(t *testing.T) {
	app := newTestApp(t)
	if w := do(app.router, http.MethodGet, "/vehiculos", "", ""); w.Body.String() != "[]" {
		t.Errorf("catálogo vacío = %s; quería []", w.Body)
	}
}

func TestReportSaleOnlyByOwner(t *testing.T) {
	app := newTestApp(t)
	owner := app.addUser(t, "ana", user.RoleVendedor)
	other := app.addUser(t, "beto", user.RoleVendedor)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleBody, owner).Body.Bytes())
	app.approve(t, v.ID)
	path := "/vehiculos/" + v.ID + "/venta"

	cases := []struct {
		name, auth string
		want       int
	}{
		{"visitante", "", http.StatusUnauthorized},
		{"otro vendedor", other, http.StatusForbidden},
		{"administrador", admin, http.StatusForbidden},
		{"dueño", owner, http.StatusOK},
		{"dueño otra vez", owner, http.StatusConflict},
	}
	for _, tc := range cases {
		if w := do(app.router, http.MethodPost, path, "", tc.auth); w.Code != tc.want {
			t.Errorf("%s: status %d; quería %d (cuerpo %s)", tc.name, w.Code, tc.want, w.Body)
		}
	}
	if w := do(app.router, http.MethodGet, "/vehiculos/"+v.ID, "", ""); w.Code != http.StatusNotFound {
		t.Errorf("el vendido no debe verse en el catálogo: status %d", w.Code)
	}
	if w := do(app.router, http.MethodPost, "/vehiculos/no-existe/venta", "", owner); w.Code != http.StatusNotFound {
		t.Errorf("vehículo inexistente: status %d; quería 404", w.Code)
	}
}

func TestSellerListsOwnVehicles(t *testing.T) {
	app := newTestApp(t)
	ana := app.addUser(t, "ana", user.RoleVendedor)
	beto := app.addUser(t, "beto", user.RoleVendedor)
	do(app.router, http.MethodPost, "/vehiculos", vehicleBody, ana)
	do(app.router, http.MethodPost, "/vehiculos", vehicleBody, beto)

	w := do(app.router, http.MethodGet, "/vendedor/vehiculos", "", ana)
	if list := decode[[]vehicleResponse](t, w.Body.Bytes()); w.Code != http.StatusOK || len(list) != 1 {
		t.Errorf("status %d, lista %+v; quería solo la de Ana", w.Code, list)
	}
	if w := do(app.router, http.MethodGet, "/vendedor/vehiculos", "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("visitante: status %d; quería 401", w.Code)
	}
}

// Un token bien firmado de un usuario que ya no existe (p. ej. eliminado por
// el administrador) no sirve: identidad y rol se leen del repositorio.
func TestTokenOfDeletedUserIsRejected(t *testing.T) {
	app := newTestApp(t)
	token, _ := app.tokens.Issue("fantasma") // token válido de un usuario que no existe
	if w := do(app.router, http.MethodPost, "/vehiculos", vehicleBody, "Bearer "+token); w.Code != http.StatusUnauthorized {
		t.Errorf("status %d; quería 401", w.Code)
	}
}
