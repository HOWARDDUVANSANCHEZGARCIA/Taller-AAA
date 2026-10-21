package user

import (
	"slices"
	"strings"
	"sync"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/jsonfile"
)

// FileRepository guarda los usuarios en un archivo JSON. Todo el acceso está
// protegido con un mutex y cada escritura reemplaza el archivo de forma atómica.
type FileRepository struct {
	mu    sync.RWMutex
	path  string
	users []User
}

var _ Repository = (*FileRepository)(nil)

// NewFileRepository abre (o crea al primer guardado) el archivo indicado.
func NewFileRepository(path string) (*FileRepository, error) {
	r := &FileRepository{path: path}
	if err := jsonfile.Load(path, &r.users); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *FileRepository) Create(u User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.users {
		if strings.EqualFold(existing.Email, u.Email) {
			return ErrEmailTaken
		}
	}
	r.users = append(r.users, u)
	if err := r.persist(); err != nil {
		r.users = r.users[:len(r.users)-1]
		return err
	}
	return nil
}

func (r *FileRepository) FindByEmail(email string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *FileRepository) FindByID(id string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *FileRepository) List() ([]User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.users), nil
}

func (r *FileRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	i := slices.IndexFunc(r.users, func(u User) bool { return u.ID == id })
	if i < 0 {
		return ErrNotFound
	}
	previous := r.users
	r.users = slices.Delete(slices.Clone(r.users), i, i+1)
	if err := r.persist(); err != nil {
		r.users = previous
		return err
	}
	return nil
}

func (r *FileRepository) persist() error {
	return jsonfile.Save(r.path, r.users)
}
