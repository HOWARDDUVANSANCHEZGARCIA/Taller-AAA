package user

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileRepositoryPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "users.json")
	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(User{ID: "1", Email: "a@x.com", Rol: RoleVendedor}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewFileRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	if u, err := reloaded.FindByID("1"); err != nil || u.Email != "a@x.com" {
		t.Errorf("FindByID tras recargar = %+v, %v", u, err)
	}
	if u, err := reloaded.FindByEmail("A@X.COM"); err != nil || u.ID != "1" {
		t.Errorf("FindByEmail tras recargar = %+v, %v", u, err)
	}
}

func TestFileRepositoryErrors(t *testing.T) {
	repo, _ := NewFileRepository(filepath.Join(t.TempDir(), "users.json"))
	_ = repo.Create(User{ID: "1", Email: "a@x.com"})

	if err := repo.Create(User{ID: "2", Email: "A@x.com"}); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("correo repetido: err = %v", err)
	}
	if _, err := repo.FindByID("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID inexistente: err = %v", err)
	}
	if _, err := repo.FindByEmail("nope@x.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByEmail inexistente: err = %v", err)
	}
}

func TestFileRepositoryConcurrentCreate(t *testing.T) {
	repo, _ := NewFileRepository(filepath.Join(t.TempDir(), "users.json"))
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- repo.Create(User{ID: "x", Email: "mismo@x.com"})
		}()
	}
	wg.Wait()
	close(results)

	created := 0
	for err := range results {
		if err == nil {
			created++
		}
	}
	if created != 1 {
		t.Errorf("se crearon %d usuarios con el mismo correo; quería 1", created)
	}
}

func TestHasRole(t *testing.T) {
	u := User{Rol: RoleVendedor}
	if !u.HasRole(RoleVendedor) || !u.HasRole(RoleAdministrador, RoleVendedor) {
		t.Error("el vendedor debe tener el rol vendedor")
	}
	if u.HasRole(RoleAdministrador) || u.HasRole() {
		t.Error("el vendedor no debe tener otros roles")
	}
}

func TestFileRepositoryListAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	repo, _ := NewFileRepository(path)
	_ = repo.Create(User{ID: "1", Email: "a@x.com"})
	_ = repo.Create(User{ID: "2", Email: "b@x.com"})

	if err := repo.Delete("1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete("1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("borrar dos veces: err = %v; quería ErrNotFound", err)
	}

	reloaded, _ := NewFileRepository(path)
	list, _ := reloaded.List()
	if len(list) != 1 || list[0].ID != "2" {
		t.Errorf("tras borrar y recargar = %+v", list)
	}
	if _, err := reloaded.FindByEmail("a@x.com"); !errors.Is(err, ErrNotFound) {
		t.Error("el correo del usuario borrado no debe existir")
	}
}
