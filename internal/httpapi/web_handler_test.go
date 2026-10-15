package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// postForm envía un formulario HTML, opcionalmente con la cookie de sesión.
func postForm(app *testApp, path string, form url.Values, session *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if session != nil {
		req.AddCookie(session)
	}
	w := httptest.NewRecorder()
	app.router.ServeHTTP(w, req)
	return w
}

func getPage(app *testApp, path string, session *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if session != nil {
		req.AddCookie(session)
	}
	w := httptest.NewRecorder()
	app.router.ServeHTTP(w, req)
	return w
}

func sessionOf(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

// webSession devuelve una cookie de sesión válida para el usuario indicado.
func (a *testApp) webSession(t *testing.T, id string, rol user.Role) *http.Cookie {
	t.Helper()
	header := a.addUser(t, id, rol)
	return &http.Cookie{Name: sessionCookie, Value: strings.TrimPrefix(header, "Bearer ")}
}

func TestWebRegisterAndLoginSetsSafeCookie(t *testing.T) {
	app := newTestApp(t)

	w := postForm(app, "/registro", url.Values{
		"nombre": {"Ana Pérez"}, "email": {"ana@correo.com"}, "telefono": {"3001234567"}, "password": {"clave-segura-1"},
	}, nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/ingresar?aviso=registrado" {
		t.Fatalf("registro: status %d, Location %q, cuerpo %s", w.Code, w.Header().Get("Location"), w.Body)
	}

	w = postForm(app, "/ingresar", url.Values{"email": {"ana@correo.com"}, "password": {"equivocada"}}, nil)
	if w.Code != http.StatusUnauthorized || sessionOf(w) != nil {
		t.Errorf("clave incorrecta: status %d; quería 401 sin cookie", w.Code)
	}

	w = postForm(app, "/ingresar", url.Values{"email": {"ana@correo.com"}, "password": {"clave-segura-1"}}, nil)
	cookie := sessionOf(w)
	if w.Code != http.StatusSeeOther || cookie == nil {
		t.Fatalf("login: status %d, cookie %v", w.Code, cookie)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("la cookie debe ser HttpOnly y SameSite=Strict: %+v", cookie)
	}
	if _, err := app.authSvc.Authenticate(cookie.Value); err != nil {
		t.Errorf("la cookie debe llevar un token válido: %v", err)
	}
}

func TestWebRegisterKeepsFormButNotPasswordOnError(t *testing.T) {
	app := newTestApp(t)
	w := postForm(app, "/registro", url.Values{
		"nombre": {"Ana"}, "email": {"ana@correo.com"}, "telefono": {"12"}, "password": {"clave-secreta-123"},
	}, nil)
	body := w.Body.String()
	if w.Code != http.StatusBadRequest || !strings.Contains(body, "teléfono") {
		t.Errorf("status %d; quería 400 con el mensaje del teléfono", w.Code)
	}
	if !strings.Contains(body, `value="ana@correo.com"`) {
		t.Error("debe conservar el correo escrito")
	}
	if strings.Contains(body, "clave-secreta-123") {
		t.Error("nunca debe devolver la contraseña en la página")
	}
}

func TestWebSellerPagesRequireSellerRole(t *testing.T) {
	app := newTestApp(t)
	admin := app.webSession(t, "admin", user.RoleAdministrador)

	for _, path := range []string{"/publicar", "/mis-vehiculos"} {
		w := getPage(app, path, nil)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/ingresar" {
			t.Errorf("visitante en %s: status %d, Location %q", path, w.Code, w.Header().Get("Location"))
		}
		if w := getPage(app, path, admin); w.Code != http.StatusForbidden {
			t.Errorf("administrador en %s: status %d; quería 403", path, w.Code)
		}
	}

	forged := &http.Cookie{Name: sessionCookie, Value: "token-falso"}
	w := postForm(app, "/publicar", url.Values{"marca": {"Kia"}}, forged)
	if w.Code != http.StatusSeeOther {
		t.Errorf("cookie falsa: status %d; quería redirección a ingresar", w.Code)
	}
	if all, _ := app.vehicles.List(); len(all) != 0 {
		t.Error("no debe crearse nada sin sesión válida")
	}
}

func TestWebPublishAndReportSale(t *testing.T) {
	app := newTestApp(t)
	ana := app.webSession(t, "ana", user.RoleVendedor)
	beto := app.webSession(t, "beto", user.RoleVendedor)

	w := postForm(app, "/publicar", url.Values{
		"marca": {"Renault"}, "modelo": {"Koleos"}, "anio": {"2019"}, "precio": {"78.000.000"}, "kilometraje": {"60 000"},
	}, ana)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("publicar: status %d, cuerpo %s", w.Code, w.Body)
	}
	own, _ := app.vehicles.List()
	if len(own) != 1 || own[0].Estado != vehicle.StatusPendiente || own[0].Precio != 78_000_000 || own[0].Kilometraje != 60_000 || own[0].VendedorID != "ana" {
		t.Fatalf("guardado = %+v", own)
	}
	id := own[0].ID

	if w := postForm(app, "/mis-vehiculos/"+id+"/vendido", nil, beto); w.Code != http.StatusForbidden {
		t.Errorf("otro vendedor: status %d; quería 403", w.Code)
	}
	if w := postForm(app, "/mis-vehiculos/"+id+"/vendido", nil, ana); w.Code != http.StatusSeeOther {
		t.Errorf("dueño: status %d", w.Code)
	}
	if v, _ := app.vehicles.FindByID(id); v.Estado != vehicle.StatusVendida {
		t.Errorf("estado = %q; quería vendida", v.Estado)
	}
}

func TestWebPublishShowsValidationErrors(t *testing.T) {
	app := newTestApp(t)
	ana := app.webSession(t, "ana", user.RoleVendedor)

	for name, form := range map[string]url.Values{
		"año en texto":    {"marca": {"Kia"}, "modelo": {"Picanto"}, "anio": {"dos mil"}, "precio": {"1"}, "kilometraje": {"1"}},
		"precio negativo": {"marca": {"Kia"}, "modelo": {"Picanto"}, "anio": {"2019"}, "precio": {"-5"}, "kilometraje": {"1"}},
		"sin marca":       {"marca": {""}, "modelo": {"Picanto"}, "anio": {"2019"}, "precio": {"1"}, "kilometraje": {"1"}},
	} {
		w := postForm(app, "/publicar", form, ana)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `class="error"`) {
			t.Errorf("%s: status %d; quería 400 con mensaje", name, w.Code)
		}
	}
}

func TestWebCatalogEscapesUserContent(t *testing.T) {
	app := newTestApp(t)
	app.addUser(t, "ana", user.RoleVendedor)
	v, err := vehicle.NewService(app.vehicles, app.users).Publish("ana", vehicle.PublishInput{
		Marca: "<script>alert(1)</script>", Modelo: "X", Anio: 2020, Precio: 1, Kilometraje: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	app.approve(t, v.ID)

	for _, path := range []string{"/", "/catalogo/" + v.ID} {
		w := getPage(app, path, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
			t.Errorf("%s: el contenido del usuario debe escaparse", path)
		}
		if !strings.Contains(w.Body.String(), "Usuario ana") {
			t.Errorf("%s: debe mostrar el contacto del vendedor", path)
		}
	}
}

func TestWebCatalogHidesPendingVehicle(t *testing.T) {
	app := newTestApp(t)
	app.addUser(t, "ana", user.RoleVendedor)
	v, _ := vehicle.NewService(app.vehicles, app.users).Publish("ana", vehicle.PublishInput{
		Marca: "Pendiente", Modelo: "X", Anio: 2020, Precio: 1,
	})

	if strings.Contains(getPage(app, "/", nil).Body.String(), "Pendiente X") {
		t.Error("el catálogo no debe mostrar publicaciones pendientes")
	}
	if w := getPage(app, "/catalogo/"+v.ID, nil); w.Code != http.StatusNotFound {
		t.Errorf("detalle de pendiente: status %d; quería 404", w.Code)
	}
}

func TestWebLogoutClearsCookie(t *testing.T) {
	app := newTestApp(t)
	ana := app.webSession(t, "ana", user.RoleVendedor)
	w := postForm(app, "/salir", nil, ana)
	if c := sessionOf(w); w.Code != http.StatusSeeOther || c == nil || c.MaxAge >= 0 {
		t.Errorf("salir: status %d, cookie %+v; quería borrarla", w.Code, c)
	}
}

func TestStylesheetIsServed(t *testing.T) {
	app := newTestApp(t)
	w := getPage(app, "/static/estilos.css", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "text/css") {
		t.Errorf("status %d, tipo %q", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestThousandsFormat(t *testing.T) {
	cases := map[int64]string{0: "0", 999: "999", 1000: "1.000", 78000000: "78.000.000", -1500: "-1.500"}
	for n, want := range cases {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q; quería %q", n, got, want)
		}
	}
	if got := telURL("+57 (300) 123-4567"); got != "tel:+573001234567" {
		t.Errorf("telURL = %q", got)
	}
}
