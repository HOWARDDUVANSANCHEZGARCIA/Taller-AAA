package vehicle

import (
	"slices"
	"sync"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/jsonfile"
)

// FileRepository guarda los vehículos en un archivo JSON, con el mismo
// esquema que el repositorio de usuarios: mutex y escritura atómica.
type FileRepository struct {
	mu       sync.RWMutex
	path     string
	vehicles []Vehicle
}

var _ Repository = (*FileRepository)(nil)

// NewFileRepository abre (o crea al primer guardado) el archivo indicado.
func NewFileRepository(path string) (*FileRepository, error) {
	r := &FileRepository{path: path}
	if err := jsonfile.Load(path, &r.vehicles); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *FileRepository) Create(v Vehicle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.vehicles = append(r.vehicles, v)
	if err := jsonfile.Save(r.path, r.vehicles); err != nil {
		r.vehicles = r.vehicles[:len(r.vehicles)-1]
		return err
	}
	return nil
}

func (r *FileRepository) FindByID(id string) (Vehicle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if i := r.index(id); i >= 0 {
		return r.vehicles[i], nil
	}
	return Vehicle{}, ErrNotFound
}

func (r *FileRepository) List() ([]Vehicle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := slices.Clone(r.vehicles)
	slices.Reverse(list) // se agregan al final: el último es el más reciente
	return list, nil
}

func (r *FileRepository) Update(id string, change func(*Vehicle) error) (Vehicle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	i := r.index(id)
	if i < 0 {
		return Vehicle{}, ErrNotFound
	}
	updated := r.vehicles[i]
	if err := change(&updated); err != nil {
		return Vehicle{}, err
	}

	previous := r.vehicles[i]
	r.vehicles[i] = updated
	if err := jsonfile.Save(r.path, r.vehicles); err != nil {
		r.vehicles[i] = previous
		return Vehicle{}, err
	}
	return updated, nil
}

func (r *FileRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	i := r.index(id)
	if i < 0 {
		return ErrNotFound
	}
	return r.replace(slices.Delete(slices.Clone(r.vehicles), i, i+1))
}

func (r *FileRepository) DeleteBySeller(sellerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	kept := slices.DeleteFunc(slices.Clone(r.vehicles), func(v Vehicle) bool { return v.VendedorID == sellerID })
	if len(kept) == len(r.vehicles) {
		return nil
	}
	return r.replace(kept)
}

// replace guarda la lista nueva y solo la adopta si se pudo escribir.
// Requiere tener el bloqueo.
func (r *FileRepository) replace(vehicles []Vehicle) error {
	if err := jsonfile.Save(r.path, vehicles); err != nil {
		return err
	}
	r.vehicles = vehicles
	return nil
}

// index devuelve la posición del vehículo o -1. Requiere tener el bloqueo.
func (r *FileRepository) index(id string) int {
	return slices.IndexFunc(r.vehicles, func(v Vehicle) bool { return v.ID == id })
}
