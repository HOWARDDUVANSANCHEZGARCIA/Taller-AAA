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

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/accounting"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/imagestore"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/testimg"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// testApp es la aplicación completa (NewRouter) con datos en un directorio temporal.
type testApp struct {
	router     *gin.Engine
	users      *user.FileRepository
	vehicles   *vehicle.FileRepository
	tokens     *auth.TokenManager
	authSvc    *auth.Service
	vehicleSvc *vehicle.Service
	logPath    string // registro de accounting
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
	vehicleSvc := vehicle.NewService(vehicles, users, imagestore.New(filepath.Join(dir, "imagenes")))
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
		logPath:    logPath,
		users:      users,
		vehicles:   vehicles,
		tokens:     tokens,
		authSvc:    authSvc,
		vehicleSvc: vehicleSvc,
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
		if w := do(app.router, http.MethodPost, "/vehiculos", vehicleForm, tc.auth); w.Code != tc.want {
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
	fields := standardFields()
	fields["estado"] = "publicada"
	fields["vendedor_id"] = "otro"
	w := sendMultipart(app.router, "/vehiculos", fields, twoImages(), seller, nil)
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
	if len(res.Imagenes) != 2 || !strings.HasPrefix(res.Imagenes[0], "/vehiculos/"+res.ID+"/imagenes/") {
		t.Errorf("imagenes = %v; quería dos URLs del vehículo", res.Imagenes)
	}
}

func TestPublishRejectsInvalidForms(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	with := func(change func(map[string]string)) map[string]string {
		f := standardFields()
		change(f)
		return f
	}

	cases := []struct {
		name   string
		fields map[string]string
		images [][]byte
		want   int
	}{
		{"precio en texto", with(func(f map[string]string) { f["precio"] = "caro" }), twoImages(), http.StatusBadRequest},
		{"sin marca", with(func(f map[string]string) { f["marca"] = "" }), twoImages(), http.StatusBadRequest},
		{"sin fotos", standardFields(), nil, http.StatusBadRequest},
		{"nueve fotos", standardFields(), append(twoImages(), make([][]byte, 7)...), http.StatusBadRequest},
		{"archivo que no es imagen", standardFields(), [][]byte{[]byte("<script>alert(1)</script>")}, http.StatusBadRequest},
		{"foto de más de 5 MB", standardFields(), [][]byte{make([]byte, imagestore.MaxBytes+1)}, http.StatusBadRequest},
		{"formulario gigante", standardFields(), make([][]byte, 9), http.StatusRequestEntityTooLarge},
	}
	cases[len(cases)-1].images = func() [][]byte {
		big := make([][]byte, 9)
		for i := range big {
			big[i] = make([]byte, imagestore.MaxBytes)
		}
		return big
	}()
	for _, tc := range cases {
		if w := sendMultipart(app.router, "/vehiculos", tc.fields, tc.images, seller, nil); w.Code != tc.want {
			t.Errorf("%s: status %d; quería %d (%s)", tc.name, w.Code, tc.want, w.Body)
		}
	}
	if w := do(app.router, http.MethodPost, "/vehiculos", `{"marca":"Renault"}`, seller); w.Code != http.StatusBadRequest {
		t.Errorf("JSON en vez de multipart: status %d; quería 400", w.Code)
	}
	if all, _ := app.vehicles.List(); len(all) != 0 {
		t.Errorf("no debe guardarse ninguna publicación inválida; hay %d", len(all))
	}
}

// Las fotos siguen la regla AAA del catálogo: públicas solo cuando la
// publicación está publicada; antes, solo para su dueño y el administrador.
func TestImageAccess(t *testing.T) {
	app := newTestApp(t)
	owner := app.addUser(t, "ana", user.RoleVendedor)
	other := app.addUser(t, "beto", user.RoleVendedor)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, owner).Body.Bytes())
	img := v.Imagenes[0]

	get := func(path, auth string) *httptest.ResponseRecorder {
		return do(app.router, http.MethodGet, path, "", auth)
	}
	for name, tc := range map[string]struct {
		auth string
		want int
	}{
		"visitante":     {"", http.StatusNotFound},
		"otro vendedor": {other, http.StatusNotFound},
		"dueño":         {owner, http.StatusOK},
		"administrador": {admin, http.StatusOK},
	} {
		if w := get(img, tc.auth); w.Code != tc.want {
			t.Errorf("pendiente, %s: status %d; quería %d", name, w.Code, tc.want)
		}
	}

	app.approve(t, v.ID)
	w := get(img, "")
	if w.Code != http.StatusOK {
		t.Fatalf("publicada, visitante: status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q; quería image/png", ct)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("falta X-Content-Type-Options: nosniff")
	}
	for _, bad := range []string{
		"/vehiculos/" + v.ID + "/imagenes/..%2F..%2Fusers.json",
		"/vehiculos/" + v.ID + "/imagenes/0123456789abcdef0123456789abcdef.png",
		"/vehiculos/no-existe/imagenes/x.png",
	} {
		if w := get(bad, owner); w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d; quería 404", bad, w.Code)
		}
	}
}

func TestVisitorSeesOnlyPublishedVehiclesWithContact(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	pending := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())
	published := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())
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
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, owner).Body.Bytes())
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
	do(app.router, http.MethodPost, "/vehiculos", vehicleForm, ana)
	do(app.router, http.MethodPost, "/vehiculos", vehicleForm, beto)

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
	if w := do(app.router, http.MethodPost, "/vehiculos", vehicleForm, "Bearer "+token); w.Code != http.StatusUnauthorized {
		t.Errorf("status %d; quería 401", w.Code)
	}
}

func TestSellerChangesPhotosOnlyWhilePending(t *testing.T) {
	app := newTestApp(t)
	owner := app.addUser(t, "ana", user.RoleVendedor)
	other := app.addUser(t, "beto", user.RoleVendedor)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, owner).Body.Bytes())
	add := "/vehiculos/" + v.ID + "/imagenes"

	if w := sendMultipart(app.router, add, nil, [][]byte{testimg.PNG()}, "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("visitante: status %d; quería 401", w.Code)
	}
	if w := sendMultipart(app.router, add, nil, [][]byte{testimg.PNG()}, other, nil); w.Code != http.StatusForbidden {
		t.Errorf("otro vendedor: status %d; quería 403", w.Code)
	}
	w := sendMultipart(app.router, add, nil, [][]byte{testimg.PNG()}, owner, nil)
	res := decode[vehicleResponse](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(res.Imagenes) != 3 {
		t.Fatalf("agregar: status %d, %v", w.Code, res.Imagenes)
	}

	name := res.Imagenes[0][strings.LastIndex(res.Imagenes[0], "/")+1:]
	if w := do(app.router, http.MethodDelete, add+"/"+name, "", other); w.Code != http.StatusForbidden {
		t.Errorf("otro vendedor quita: status %d; quería 403", w.Code)
	}
	w = do(app.router, http.MethodDelete, add+"/"+name, "", owner)
	if res := decode[vehicleResponse](t, w.Body.Bytes()); w.Code != http.StatusOK || len(res.Imagenes) != 2 {
		t.Fatalf("quitar: status %d, %v", w.Code, res.Imagenes)
	}

	app.approve(t, v.ID)
	if w := sendMultipart(app.router, add, nil, [][]byte{testimg.PNG()}, owner, nil); w.Code != http.StatusConflict {
		t.Errorf("agregar tras aprobar: status %d; quería 409", w.Code)
	}
	stored, _ := app.vehicles.FindByID(v.ID)
	if w := do(app.router, http.MethodDelete, add+"/"+stored.Imagenes[0], "", owner); w.Code != http.StatusConflict {
		t.Errorf("quitar tras aprobar: status %d; quería 409", w.Code)
	}
}
