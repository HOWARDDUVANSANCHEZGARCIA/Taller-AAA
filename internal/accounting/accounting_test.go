package accounting

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var key = []byte("clave-de-accounting")

func writeEntries(t *testing.T, path string, n int) {
	t.Helper()
	l, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := 0; i < n; i++ {
		if err := l.Record(Entry{Operacion: "GET /vehiculos", Recurso: "/vehiculos", Actor: "anonimo", Rol: "visitante", Resultado: 200, IP: "127.0.0.1"}); err != nil {
			t.Fatal(err)
		}
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestChainContinuesAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "accounting.log")
	writeEntries(t, path, 2)
	writeEntries(t, path, 3) // se reabre: la cadena debe continuar

	n, err := Verify(path, key)
	if err != nil || n != 5 {
		t.Fatalf("Verify = %d, %v; quería 5 entradas íntegras", n, err)
	}
	// Windows no maneja permisos tipo Unix (siempre reporta rw-rw-rw-), así
	// que esta comprobación solo aplica en Linux y macOS.
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("permisos = %v; quería 0600", info.Mode().Perm())
	}
}

func TestDetectsTampering(t *testing.T) {
	cases := map[string]func([]string) []string{
		"cambiar el resultado": func(l []string) []string {
			l[1] = strings.Replace(l[1], `"resultado":200`, `"resultado":403`, 1)
			return l
		},
		"cambiar el actor": func(l []string) []string {
			l[2] = strings.Replace(l[2], `"actor":"anonimo"`, `"actor":"otro"`, 1)
			return l
		},
		"borrar una línea del medio":   func(l []string) []string { return append(l[:1], l[2:]...) },
		"borrar la primera línea":      func(l []string) []string { return l[1:] },
		"intercambiar dos líneas":      func(l []string) []string { l[1], l[2] = l[2], l[1]; return l },
		"insertar una línea duplicada": func(l []string) []string { return append(l[:2], append([]string{l[1]}, l[2:]...)...) },
		"línea basura":                 func(l []string) []string { l[3] = "{no es json"; return l },
	}
	for name, tamper := range cases {
		path := filepath.Join(t.TempDir(), "accounting.log")
		writeEntries(t, path, 4)
		writeLines(t, path, tamper(readLines(t, path)))

		var terr *TamperError
		if _, err := Verify(path, key); !errors.As(err, &terr) {
			t.Errorf("%s: err = %v; quería TamperError", name, err)
		}
		if _, err := Open(path, key); !errors.As(err, &terr) {
			t.Errorf("%s: Open debe negarse a abrir un registro alterado", name)
		}
	}
}

func TestWrongKeyIsDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounting.log")
	writeEntries(t, path, 2)
	var terr *TamperError
	if _, err := Verify(path, []byte("otra-clave")); !errors.As(err, &terr) || terr.Line != 1 {
		t.Errorf("err = %v; quería TamperError en la línea 1", err)
	}
}

func TestEmptyKeyIsRejected(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "a.log"), nil); err == nil {
		t.Error("sin clave no se puede garantizar la integridad")
	}
}

func TestConcurrentRecordsKeepChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounting.log")
	l, _ := Open(path, key)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.Record(Entry{Operacion: "GET /", Resultado: 200})
		}()
	}
	wg.Wait()
	l.Close()

	if n, err := Verify(path, key); err != nil || n != 50 {
		t.Errorf("Verify = %d, %v; quería 50 íntegras", n, err)
	}
}
