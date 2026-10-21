// Package imagestore guarda las fotos de los vehículos como archivos en
// disco, una carpeta por vehículo. En los datos del vehículo (JSON) solo
// queda la lista de nombres de archivo.
//
// Seguridad:
//   - El formato se comprueba leyendo el contenido (cabecera JPG o PNG), no
//     el nombre ni el Content-Type que envía el cliente.
//   - Se limitan el peso y las dimensiones, para evitar imágenes gigantes.
//   - El nombre del archivo lo genera el servidor (aleatorio); el del cliente
//     se descarta. Así nadie puede escribir ni leer fuera de la carpeta.
package imagestore

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registra el lector de JPG de la librería estándar
	_ "image/png"  // registra el lector de PNG de la librería estándar
	"os"
	"path/filepath"
	"regexp"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/ids"
)

const (
	MaxBytes = 5 << 20 // 5 MB por imagen
	maxSide  = 6000    // píxeles por lado
)

// ErrNotFound indica un nombre de imagen inválido o inexistente.
var ErrNotFound = errors.New("imagen no encontrada")

var (
	validID   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	validName = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png)$`)
	extension = map[string]string{"jpeg": ".jpg", "png": ".png"}
)

type Store struct {
	dir string
}

func New(dir string) *Store {
	return &Store{dir: dir}
}

// Save valida la imagen, la guarda en la carpeta del vehículo y devuelve el
// nombre generado.
func (s *Store) Save(vehicleID string, data []byte) (string, error) {
	if !validID.MatchString(vehicleID) {
		return "", fmt.Errorf("id de vehículo inválido: %q", vehicleID)
	}
	ext, err := check(data)
	if err != nil {
		return "", err
	}
	id, err := ids.New()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.dir, vehicleID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := id + ext
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

// Path devuelve la ruta del archivo. Solo acepta nombres con el formato que
// genera Save, así que no hay forma de salirse de la carpeta.
func (s *Store) Path(vehicleID, name string) (string, error) {
	if !validID.MatchString(vehicleID) || !validName.MatchString(name) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, vehicleID, name), nil
}

// Delete borra una imagen. Si ya no existe no es un error.
func (s *Store) Delete(vehicleID, name string) error {
	path, err := s.Path(vehicleID, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteAll borra todas las imágenes de un vehículo.
func (s *Store) DeleteAll(vehicleID string) error {
	if !validID.MatchString(vehicleID) {
		return fmt.Errorf("id de vehículo inválido: %q", vehicleID)
	}
	return os.RemoveAll(filepath.Join(s.dir, vehicleID))
}

// check confirma que data sea un JPG o PNG de tamaño razonable y devuelve
// la extensión que le corresponde.
func check(data []byte) (string, error) {
	if len(data) == 0 {
		return "", &apperr.ValidationError{Msg: "una de las imágenes está vacía"}
	}
	if len(data) > MaxBytes {
		return "", &apperr.ValidationError{Msg: "cada imagen debe pesar máximo 5 MB"}
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	ext, ok := extension[format]
	if err != nil || !ok {
		return "", &apperr.ValidationError{Msg: "solo se aceptan imágenes JPG o PNG"}
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide {
		return "", &apperr.ValidationError{Msg: "las imágenes deben medir máximo 6000 × 6000 píxeles"}
	}
	return ext, nil
}
