package admin

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

func newTestService(t *testing.T) (*Service, *user.FileRepository, *vehicle.Service) {
	t.Helper()
	dir := t.TempDir()
	users, _ := user.NewFileRepository(filepath.Join(dir, "users.json"))
	repo, _ := vehicle.NewFileRepository(filepath.Join(dir, "vehicles.json"))
	vehicles := vehicle.NewService(repo, users)
	for _, u := range []user.User{
		{ID: "admin", Nombre: "Admin", Email: "admin@x.com", Rol: user.RoleAdministrador},
		{ID: "ana", Nombre: "Ana", Email: "ana@x.com", Rol: user.RoleVendedor},
		{ID: "beto", Nombre: "Beto", Email: "beto@x.com", Rol: user.RoleVendedor},
	} {
		if err := users.Create(u); err != nil {
			t.Fatal(err)
		}
	}
	return NewService(users, vehicles), users, vehicles
}

func publish(t *testing.T, vehicles *vehicle.Service, seller string) vehicle.Vehicle {
	t.Helper()
	v, err := vehicles.Publish(seller, vehicle.PublishInput{Marca: "Kia", Modelo: "Picanto", Anio: 2020, Precio: 1})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestApproveMakesVehicleVisible(t *testing.T) {
	svc, _, vehicles := newTestService(t)
	v := publish(t, vehicles, "ana")

	if _, err := svc.Approve(v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := vehicles.CatalogItem(v.ID); err != nil {
		t.Errorf("la aprobada debe verse en el catálogo: %v", err)
	}
}

func TestPublicationsIncludeAllStatesWithSeller(t *testing.T) {
	svc, _, vehicles := newTestService(t)
	pending := publish(t, vehicles, "ana")
	approved := publish(t, vehicles, "beto")
	_, _ = svc.Approve(approved.ID)

	pubs, err := svc.Publications()
	if err != nil || len(pubs) != 2 {
		t.Fatalf("Publications = %+v, %v", pubs, err)
	}
	if pubs[0].ID != approved.ID || pubs[0].Vendedor != "Beto" || pubs[1].ID != pending.ID || pubs[1].Estado != vehicle.StatusPendiente {
		t.Errorf("Publications = %+v", pubs)
	}
}

func TestDeletePublication(t *testing.T) {
	svc, _, vehicles := newTestService(t)
	v := publish(t, vehicles, "ana")
	_, _ = svc.Approve(v.ID)

	if err := svc.DeletePublication(v.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := vehicles.Catalog(); len(list) != 0 {
		t.Error("la publicación eliminada no debe verse")
	}
	if err := svc.DeletePublication(v.ID); !errors.Is(err, vehicle.ErrNotFound) {
		t.Errorf("eliminar dos veces: err = %v", err)
	}
}

func TestDeleteUserRemovesUserAndPublications(t *testing.T) {
	svc, users, vehicles := newTestService(t)
	publish(t, vehicles, "ana")
	publish(t, vehicles, "ana")
	keep := publish(t, vehicles, "beto")

	if err := svc.DeleteUser("admin", "ana"); err != nil {
		t.Fatal(err)
	}
	if _, err := users.FindByID("ana"); !errors.Is(err, user.ErrNotFound) {
		t.Error("el usuario debe quedar eliminado")
	}
	all, _ := vehicles.All()
	if len(all) != 1 || all[0].ID != keep.ID {
		t.Errorf("quedan %+v; quería solo la de Beto", all)
	}
	if err := svc.DeleteUser("admin", "ana"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("eliminar dos veces: err = %v", err)
	}
	if err := svc.DeleteUser("admin", "admin"); !errors.Is(err, ErrSelfDelete) {
		t.Errorf("auto-eliminación: err = %v; quería ErrSelfDelete", err)
	}
}
