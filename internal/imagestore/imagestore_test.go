package imagestore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
)

const vehicleID = "0123456789abcdef0123456789abcdef"

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 3)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// hugePNG es un PNG válido que declara 10000 × 10000 píxeles en su cabecera.
func hugePNG(t *testing.T) []byte {
	data := pngBytes(t, 1, 1)
	binary.BigEndian.PutUint32(data[16:], 10000)
	binary.BigEndian.PutUint32(data[20:], 10000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func TestSaveAcceptsJPGAndPNG(t *testing.T) {
	s := New(t.TempDir())
	for name, data := range map[string][]byte{".png": pngBytes(t, 4, 3), ".jpg": jpegBytes(t)} {
		got, err := s.Save(vehicleID, data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasSuffix(got, name) || !validName.MatchString(got) {
			t.Errorf("nombre = %q; quería un nombre aleatorio terminado en %s", got, name)
		}
		path, err := s.Path(vehicleID, got)
		if err != nil {
			t.Fatal(err)
		}
		if stored, _ := os.ReadFile(path); !bytes.Equal(stored, data) {
			t.Errorf("%s: el archivo guardado no coincide", name)
		}
	}
}

func TestSaveRejectsInvalidImages(t *testing.T) {
	s := New(t.TempDir())
	cases := map[string][]byte{
		"vacía":                         nil,
		"texto con extensión de imagen": []byte("<script>alert(1)</script>"),
		"GIF":                           []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"demasiado pesada":              append(pngBytes(t, 1, 1), make([]byte, MaxBytes)...),
		"dimensiones enormes":           hugePNG(t),
	}
	for name, data := range cases {
		var verr *apperr.ValidationError
		if _, err := s.Save(vehicleID, data); !errors.As(err, &verr) {
			t.Errorf("%s: err = %v; quería ValidationError", name, err)
		}
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	s := New(t.TempDir())
	for _, tc := range []struct{ id, name string }{
		{vehicleID, "../../users.json"},
		{vehicleID, "foto.png"},
		{"../..", "0123456789abcdef0123456789abcdef.png"},
		{vehicleID, "0123456789abcdef0123456789abcdef.png/../x"},
	} {
		if _, err := s.Path(tc.id, tc.name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Path(%q, %q) debe rechazarse", tc.id, tc.name)
		}
	}
	if _, err := s.Save("../fuera", pngBytes(t, 1, 1)); err == nil {
		t.Error("Save no debe aceptar un id de vehículo con ruta")
	}
}

func TestDeleteAll(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_, _ = s.Save(vehicleID, pngBytes(t, 1, 1))
	_, _ = s.Save(vehicleID, pngBytes(t, 1, 1))

	if err := s.DeleteAll(vehicleID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, vehicleID)); !os.IsNotExist(err) {
		t.Error("la carpeta del vehículo debe borrarse")
	}
	if err := s.DeleteAll(vehicleID); err != nil {
		t.Errorf("borrar algo que ya no existe no es un error: %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := New(t.TempDir())
	name, _ := s.Save(vehicleID, pngBytes(t, 1, 1))
	if err := s.Delete(vehicleID, name); err != nil {
		t.Fatal(err)
	}
	path, _ := s.Path(vehicleID, name)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("la imagen debe borrarse")
	}
	if err := s.Delete(vehicleID, name); err != nil {
		t.Errorf("borrar dos veces no es un error: %v", err)
	}
	if err := s.Delete(vehicleID, "../../users.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nombre inválido: err = %v", err)
	}
}
