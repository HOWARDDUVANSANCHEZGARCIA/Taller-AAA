package jsonfile

import (
	"os"
	"path/filepath"
	"testing"
)

type item struct {
	Nombre string `json:"nombre"`
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "datos.json")
	if err := Save(path, []item{{"a"}, {"b"}}); err != nil {
		t.Fatal(err)
	}

	var got []item
	if err := Load(path, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Nombre != "b" {
		t.Errorf("Load = %+v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("no debe quedar el archivo temporal")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("permisos = %v; quería 0600", info.Mode().Perm())
	}
}

func TestLoadMissingOrEmptyFile(t *testing.T) {
	dir := t.TempDir()
	var got []item
	if err := Load(filepath.Join(dir, "no-existe.json"), &got); err != nil || got != nil {
		t.Errorf("archivo inexistente: %v, %+v", err, got)
	}

	empty := filepath.Join(dir, "vacio.json")
	_ = os.WriteFile(empty, nil, 0o600)
	if err := Load(empty, &got); err != nil || got != nil {
		t.Errorf("archivo vacío: %v, %+v", err, got)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malo.json")
	_ = os.WriteFile(path, []byte("{no es json"), 0o600)
	var got []item
	if err := Load(path, &got); err == nil {
		t.Error("un archivo corrupto debe dar error")
	}
}
