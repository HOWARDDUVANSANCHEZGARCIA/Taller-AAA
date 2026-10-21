package vehicle

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileRepositoryPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vehicles.json")
	repo, _ := NewFileRepository(path)
	_ = repo.Create(Vehicle{ID: "1", Marca: "Renault", Estado: StatusPendiente})
	_, _ = repo.Update("1", func(v *Vehicle) error { v.Estado = StatusPublicada; return nil })

	reloaded, err := NewFileRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := reloaded.FindByID("1"); err != nil || v.Estado != StatusPublicada {
		t.Errorf("tras recargar = %+v, %v", v, err)
	}
}

func TestFileRepositoryUpdate(t *testing.T) {
	repo, _ := NewFileRepository(filepath.Join(t.TempDir(), "vehicles.json"))
	_ = repo.Create(Vehicle{ID: "1", Estado: StatusPendiente})

	if _, err := repo.Update("nope", func(*Vehicle) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Errorf("inexistente: err = %v", err)
	}

	boom := errors.New("rechazado")
	if _, err := repo.Update("1", func(v *Vehicle) error { v.Estado = StatusVendida; return boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v; quería el error de change", err)
	}
	if v, _ := repo.FindByID("1"); v.Estado != StatusPendiente {
		t.Error("si change falla no debe guardarse nada")
	}
}

// Dos solicitudes simultáneas de venta: solo una debe tener éxito.
func TestFileRepositoryConcurrentUpdate(t *testing.T) {
	repo, _ := NewFileRepository(filepath.Join(t.TempDir(), "vehicles.json"))
	_ = repo.Create(Vehicle{ID: "1", Estado: StatusPublicada})

	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Update("1", func(v *Vehicle) error {
				if v.Estado == StatusVendida {
					return ErrAlreadySold
				}
				v.Estado = StatusVendida
				return nil
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Errorf("%d ventas exitosas; quería 1", ok)
	}
}
