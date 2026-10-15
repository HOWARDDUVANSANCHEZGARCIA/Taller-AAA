// Package accounting implementa la parte Accounting de RF-01: cada operación
// queda registrada de forma trazable (quién, qué, sobre qué recurso, con qué
// resultado y cuándo) e íntegra (cualquier cambio posterior se detecta).
//
// Integridad: cada línea lleva un HMAC-SHA256 de su contenido junto con el
// hash de la línea anterior. Si alguien edita, borra o reordena una línea,
// la cadena deja de cuadrar desde ese punto. Sin la clave no se puede
// recalcular la cadena para disimular el cambio.
package accounting

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry es una línea del registro.
type Entry struct {
	Seq        uint64    `json:"seq"`
	Momento    time.Time `json:"momento"`
	Operacion  string    `json:"operacion"` // método y ruta, p. ej. "POST /vehiculos/:id/venta"
	Recurso    string    `json:"recurso"`   // ruta concreta, p. ej. "/vehiculos/3f2a.../venta"
	Actor      string    `json:"actor"`     // ID del usuario, o "anonimo"
	Rol        string    `json:"rol"`       // visitante, vendedor o administrador
	Resultado  int       `json:"resultado"` // código HTTP de la respuesta
	IP         string    `json:"ip"`
	HashPrevio string    `json:"hash_previo"`
	Hash       string    `json:"hash"`
}

// TamperError indica en qué línea se rompe la cadena del registro.
type TamperError struct {
	Line   int
	Reason string
}

func (e *TamperError) Error() string {
	return fmt.Sprintf("registro de accounting alterado en la línea %d: %s", e.Line, e.Reason)
}

// Log es el registro de accounting. Solo agrega líneas al final.
type Log struct {
	mu   sync.Mutex
	file *os.File
	key  []byte
	seq  uint64
	last string
	now  func() time.Time
}

// Open verifica la cadena del archivo existente y lo deja listo para agregar.
// Si el archivo fue alterado devuelve un *TamperError y no lo abre.
func Open(path string, key []byte) (*Log, error) {
	if len(key) == 0 {
		return nil, errors.New("la clave de accounting está vacía")
	}
	seq, last, err := verify(path, key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Log{file: f, key: key, seq: seq, last: last, now: time.Now}, nil
}

// Record completa número, momento y hashes de la entrada y la escribe.
func (l *Log) Record(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	e.Seq = l.seq + 1
	e.Momento = l.now().UTC()
	e.HashPrevio = l.last
	e.Hash = sign(l.key, e)

	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := l.file.Write(append(line, '\n')); err != nil {
		return err
	}
	if err := l.file.Sync(); err != nil {
		return err
	}
	l.seq, l.last = e.Seq, e.Hash
	return nil
}

func (l *Log) Close() error { return l.file.Close() }

// Verify revisa todo el archivo y devuelve cuántas entradas tiene.
func Verify(path string, key []byte) (int, error) {
	seq, _, err := verify(path, key)
	return int(seq), err
}

func verify(path string, key []byte) (seq uint64, last string, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return 0, "", &TamperError{line, "no es una entrada válida"}
		}
		switch {
		case e.Seq != seq+1:
			return 0, "", &TamperError{line, fmt.Sprintf("se esperaba la entrada %d y aparece la %d (faltan o sobran líneas)", seq+1, e.Seq)}
		case e.HashPrevio != last:
			return 0, "", &TamperError{line, "no continúa la cadena de la línea anterior"}
		case !hmac.Equal([]byte(e.Hash), []byte(sign(key, e))):
			return 0, "", &TamperError{line, "el contenido no coincide con su firma"}
		}
		seq, last = e.Seq, e.Hash
	}
	return seq, last, scanner.Err()
}

// sign calcula el HMAC de todos los campos menos Hash. Los campos se
// serializan como un arreglo JSON para que no haya ambigüedad entre ellos.
func sign(key []byte, e Entry) string {
	data, _ := json.Marshal([]any{
		e.Seq, e.Momento.Format(time.RFC3339Nano), e.Operacion, e.Recurso,
		e.Actor, e.Rol, e.Resultado, e.IP, e.HashPrevio,
	})
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
