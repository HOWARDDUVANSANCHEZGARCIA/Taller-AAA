// Package jsonfile guarda y carga datos en archivos JSON. Lo usan los
// repositorios de usuarios y de vehículos para no repetir la misma lógica.
package jsonfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Load lee el archivo en v. Si el archivo no existe o está vacío, deja v
// sin cambios y no devuelve error: el archivo se crea al primer Save.
func Load(path string, v any) error {
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("leyendo %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("interpretando %s: %w", path, err)
	}
	return nil
}

// Save escribe v en un archivo temporal y lo renombra, para que un fallo a
// mitad de escritura no deje el archivo original corrupto. El archivo queda
// legible solo por el usuario que ejecuta el servidor.
func Save(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
