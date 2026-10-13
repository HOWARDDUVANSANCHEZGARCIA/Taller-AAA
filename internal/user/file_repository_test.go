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
