package httpapi

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/accounting"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

func readAccounting(t *testing.T, path string) []accounting.Entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var entries []accounting.Entry
	s := bufio.NewScanner(f)
	for s.Scan() {
		var e accounting.Entry
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return entries
}

func login(t *testing.T, app *testApp, email, password string) string {
	t.Helper()
	w := do(app.router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: status %d", email, w.Code)
	}
	return "Bearer " + decode[loginResponse](t, w.Body.Bytes()).Token
}

// Recorre las tres historias de usuario por la API y comprueba
// Authentication, Authorization y Accounting en cada paso.
func TestAAAEndToEnd(t *testing.T) {
	app := newTestApp(t)
	if err := app.authSvc.EnsureAdmin("Admin", "admin@automarket.com", "clave-admin-123"); err != nil {
		t.Fatal(err)
	}
	adminUser, _ := app.users.FindByEmail("admin@automarket.com")

	type step struct {
		name, method, path, body, auth string
		want                           int
		actor                          string // "" = anonimo
		rol                            string
	}
	var anaID, vehicleID, ana, admin string

	steps := []func() step{
		func() step {
			return step{"visitante ve el catálogo", "GET", "/vehiculos", "", "", 200, "", "visitante"}
		},
		func() step {
			return step{"Ana se registra", "POST", "/auth/register",
				`{"nombre":"Ana Pérez","email":"ana@correo.com","telefono":"3001234567","password":"clave-segura-1"}`, "", 201, "", "visitante"}
		},
		func() step {
			return step{"clave incorrecta", "POST", "/auth/login", `{"email":"ana@correo.com","password":"mala-clave"}`, "", 401, "", "visitante"}
		},
		func() step {
			ana = login(t, app, "ana@correo.com", "clave-segura-1")
			u, _ := app.users.FindByEmail("ana@correo.com")
			anaID = u.ID
			return step{"visitante intenta publicar", "POST", "/vehiculos", vehicleForm, "", 401, "", "visitante"}
		},
		func() step {
			return step{"Ana publica", "POST", "/vehiculos", vehicleForm, ana, 201, anaID, "vendedor"}
		},
		func() step {
			all, _ := app.vehicles.List()
			vehicleID = all[0].ID
			return step{"Ana intenta aprobar su propia publicación", "POST", "/admin/vehiculos/" + vehicleID + "/aprobar", "", ana, 403, anaID, "vendedor"}
		},
		func() step {
			return step{"visitante no ve la pendiente", "GET", "/vehiculos/" + vehicleID, "", "", 404, "", "visitante"}
		},
		func() step {
			admin = login(t, app, "admin@automarket.com", "clave-admin-123")
			return step{"administrador aprueba", "POST", "/admin/vehiculos/" + vehicleID + "/aprobar", "", admin, 200, adminUser.ID, "administrador"}
		},
		func() step {
			return step{"visitante ve la publicada", "GET", "/vehiculos/" + vehicleID, "", "", 200, "", "visitante"}
		},
		func() step {
			return step{"Ana reporta la venta", "POST", "/vehiculos/" + vehicleID + "/venta", "", ana, 200, anaID, "vendedor"}
		},
		func() step {
			return step{"administrador elimina a Ana", "DELETE", "/admin/usuarios/" + anaID, "", admin, 204, adminUser.ID, "administrador"}
		},
		func() step {
			return step{"el token de Ana ya no sirve", "POST", "/vehiculos", vehicleForm, ana, 401, "", "visitante"}
		},
	}

	var expected []step
	for _, next := range steps {
		s := next()
		if w := do(app.router, s.method, s.path, s.body, s.auth); w.Code != s.want {
			t.Fatalf("%s: status %d; quería %d (%s)", s.name, w.Code, s.want, w.Body)
		}
		expected = append(expected, s)
	}

	// Accounting: la cadena está íntegra y hay una entrada por solicitud,
	// incluidos los dos logins exitosos hechos con login().
	n, err := accounting.Verify(app.logPath, []byte("clave-accounting-pruebas"))
	if err != nil {
		t.Fatalf("registro alterado: %v", err)
	}
	entries := readAccounting(t, app.logPath)
	if n != len(expected)+2 || len(entries) != n {
		t.Fatalf("entradas = %d; quería %d", n, len(expected)+2)
	}

	var logins []accounting.Entry
	i := 0
	for _, e := range entries {
		if e.Operacion == "POST /auth/login" && e.Resultado == 200 {
			logins = append(logins, e)
			continue
		}
		s := expected[i]
		i++
		actor := s.actor
		if actor == "" {
			actor = "anonimo"
		}
		if e.Actor != actor || e.Rol != s.rol || e.Resultado != s.want || !strings.HasPrefix(e.Operacion, s.method+" ") || e.Recurso != s.path {
			t.Errorf("%s: entrada = %+v", s.name, e)
		}
		if e.Momento.IsZero() || e.IP == "" {
			t.Errorf("%s: falta el momento o la IP: %+v", s.name, e)
		}
	}
	if len(logins) != 2 || logins[0].Actor != anaID || logins[1].Actor != adminUser.ID {
		t.Errorf("los logins exitosos deben registrar quién ingresó: %+v", logins)
	}

	raw, _ := os.ReadFile(app.logPath)
	for _, secret := range []string{"clave-segura-1", "clave-admin-123", strings.TrimPrefix(ana, "Bearer ")} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("el registro no debe contener contraseñas ni tokens (%q)", secret)
		}
	}

	if v, err := app.vehicles.FindByID(vehicleID); err == nil {
		t.Errorf("al eliminar a Ana debe eliminarse su vehículo: %+v", v)
	}
	if _, err := app.users.FindByID(anaID); err == nil {
		t.Error("Ana debe estar eliminada")
	}
}

func TestAccountingRecordsWebPagesButNotStylesheet(t *testing.T) {
	app := newTestApp(t)
	getPage(app, "/", nil)
	getPage(app, "/static/estilos.css", nil)
	getPage(app, "/no-existe", nil)

	entries := readAccounting(t, app.logPath)
	if len(entries) != 2 {
		t.Fatalf("entradas = %+v; quería 2 (la hoja de estilos no se registra)", entries)
	}
	if entries[0].Operacion != "GET /" || entries[1].Operacion != "GET (ruta inexistente)" || entries[1].Resultado != 404 {
		t.Errorf("entradas = %+v", entries)
	}
}

// Un cliente no puede falsificar su IP en el registro con X-Forwarded-For.
func TestAccountingIgnoresForwardedFor(t *testing.T) {
	app := newTestApp(t)
	req, _ := http.NewRequest(http.MethodGet, "/vehiculos", nil)
	req.RemoteAddr = "10.0.0.7:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	app.router.ServeHTTP(httptest.NewRecorder(), req)

	if e := readAccounting(t, app.logPath); len(e) != 1 || e[0].IP != "10.0.0.7" {
		t.Errorf("IP registrada = %+v; quería la de la conexión", e)
	}
}

// Ver una foto también es una operación sobre un recurso: queda registrada.
func TestAccountingRecordsImageViews(t *testing.T) {
	app := newTestApp(t)
	owner := app.addUser(t, "ana", user.RoleVendedor)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, owner).Body.Bytes())
	do(app.router, http.MethodGet, v.Imagenes[0], "", "")
	do(app.router, http.MethodGet, v.Imagenes[0], "", owner)

	entries := readAccounting(t, app.logPath)
	last := entries[len(entries)-2:]
	if last[0].Operacion != "GET /vehiculos/:id/imagenes/:nombre" || last[0].Resultado != 404 || last[0].Actor != "anonimo" {
		t.Errorf("visitante viendo foto pendiente: %+v", last[0])
	}
	if last[1].Resultado != 200 || last[1].Actor != "ana" || last[1].Recurso != v.Imagenes[0] {
		t.Errorf("dueño viendo su foto: %+v", last[1])
	}
}
