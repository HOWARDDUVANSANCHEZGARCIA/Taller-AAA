package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return r, nil
	case err != nil:
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	if len(data) == 0 {
		return r, nil
	}
	if err := json.Unmarshal(data, &r.users); err != nil {
		return nil, fmt.Errorf("interpretando %s: %w", path, err)
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

// persist escribe a un archivo temporal y lo renombra, para que un fallo a
// mitad de escritura no deje el archivo de usuarios corrupto.
func (r *FileRepository) persist() error {
	data, err := json.MarshalIndent(r.users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
