// Package ids genera identificadores aleatorios para usuarios y vehículos.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// New devuelve 16 bytes aleatorios en hexadecimal (32 caracteres). Al ser
// aleatorios, no se pueden adivinar ni recorrer en orden.
func New() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
