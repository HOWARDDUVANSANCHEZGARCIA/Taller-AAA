package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// Ningún rol distinto del administrador puede ejecutar la moderación.
func TestAdminRoutesRequireAdministrator(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())

	routes := []struct{ method, path string }{
		{http.MethodGet, "/admin/vehiculos"},
		{http.MethodPost, "/admin/vehiculos/" + v.ID + "/aprobar"},
		{http.MethodDelete, "/admin/vehiculos/" + v.ID},
		{http.MethodGet, "/admin/usuarios"},
		{http.MethodDelete, "/admin/usuarios/ana"},
	}
	for _, rt := range routes {
		if w := do(app.router, rt.method, rt.path, "", ""); w.Code != http.StatusUnauthorized {
			t.Errorf("visitante %s %s: status %d; quería 401", rt.method, rt.path, w.Code)
		}
		if w := do(app.router, rt.method, rt.path, "", seller); w.Code != http.StatusForbidden {
			t.Errorf("vendedor %s %s: status %d; quería 403", rt.method, rt.path, w.Code)
		}
	}
	if stored, _ := app.vehicles.FindByID(v.ID); stored.Estado != vehicle.StatusPendiente {
		t.Error("los intentos rechazados no deben cambiar nada")
	}
	if _, err := app.users.FindByID("ana"); err != nil {
		t.Error("los intentos rechazados no deben borrar al usuario")
	}
}

// Criterio de aceptación de HU-ADM-01: al aprobar una publicación pendiente
// pasa a publicada y se ve en el catálogo público.
func TestAdminApprovesPendingVehicle(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())

	list := decode[[]publicationResponse](t, do(app.router, http.MethodGet, "/admin/vehiculos", "", admin).Body.Bytes())
	if len(list) != 1 || list[0].Estado != vehicle.StatusPendiente || list[0].Vendedor != "Usuario ana" {
		t.Fatalf("listado del administrador = %+v", list)
	}

	w := do(app.router, http.MethodPost, "/admin/vehiculos/"+v.ID+"/aprobar", "", admin)
	if res := decode[vehicleResponse](t, w.Body.Bytes()); w.Code != http.StatusOK || res.Estado != vehicle.StatusPublicada {
		t.Fatalf("aprobar: status %d, %+v", w.Code, res)
	}
	if w := do(app.router, http.MethodGet, "/vehiculos/"+v.ID, "", ""); w.Code != http.StatusOK {
		t.Errorf("el visitante debe ver la aprobada: status %d", w.Code)
	}
	if w := do(app.router, http.MethodPost, "/admin/vehiculos/"+v.ID+"/aprobar", "", admin); w.Code != http.StatusConflict {
		t.Errorf("aprobar dos veces: status %d; quería 409", w.Code)
	}
	if w := do(app.router, http.MethodPost, "/admin/vehiculos/no-existe/aprobar", "", admin); w.Code != http.StatusNotFound {
		t.Errorf("inexistente: status %d; quería 404", w.Code)
	}
}

func TestAdminDeletesPublication(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())
	app.approve(t, v.ID)

	if w := do(app.router, http.MethodDelete, "/admin/vehiculos/"+v.ID, "", admin); w.Code != http.StatusNoContent {
		t.Fatalf("eliminar: status %d", w.Code)
	}
	if w := do(app.router, http.MethodGet, "/vehiculos/"+v.ID, "", ""); w.Code != http.StatusNotFound {
		t.Errorf("la eliminada no debe verse: status %d", w.Code)
	}
	if w := do(app.router, http.MethodDelete, "/admin/vehiculos/"+v.ID, "", admin); w.Code != http.StatusNotFound {
		t.Errorf("eliminar dos veces: status %d; quería 404", w.Code)
	}
}

func TestAdminDeletesUserAndRevokesAccess(t *testing.T) {
	app := newTestApp(t)
	seller := app.addUser(t, "ana", user.RoleVendedor)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	v := decode[vehicleResponse](t, do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller).Body.Bytes())

	if w := do(app.router, http.MethodDelete, "/admin/usuarios/ana", "", admin); w.Code != http.StatusNoContent {
		t.Fatalf("eliminar usuario: status %d, %s", w.Code, w.Body)
	}
	if _, err := app.vehicles.FindByID(v.ID); err == nil {
		t.Error("las publicaciones del usuario también deben eliminarse")
	}
	if w := do(app.router, http.MethodPost, "/vehiculos", vehicleForm, seller); w.Code != http.StatusUnauthorized {
		t.Errorf("el token del usuario eliminado debe dejar de servir: status %d", w.Code)
	}
	users := decode[[]adminUserResponse](t, do(app.router, http.MethodGet, "/admin/usuarios", "", admin).Body.Bytes())
	if len(users) != 1 || users[0].ID != "admin" {
		t.Errorf("usuarios = %+v", users)
	}
	if w := do(app.router, http.MethodDelete, "/admin/usuarios/admin", "", admin); w.Code != http.StatusConflict {
		t.Errorf("auto-eliminación: status %d; quería 409", w.Code)
	}
	if w := do(app.router, http.MethodDelete, "/admin/usuarios/nadie", "", admin); w.Code != http.StatusNotFound {
		t.Errorf("inexistente: status %d; quería 404", w.Code)
	}
}

func TestAdminUserListNeverExposesPasswordHash(t *testing.T) {
	app := newTestApp(t)
	admin := app.addUser(t, "admin", user.RoleAdministrador)
	body := do(app.router, http.MethodGet, "/admin/usuarios", "", admin).Body.String()
	if strings.Contains(body, "password") || strings.Contains(body, "hash") {
		t.Errorf("la respuesta no debe incluir el hash: %s", body)
	}
}
