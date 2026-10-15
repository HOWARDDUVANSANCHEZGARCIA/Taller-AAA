// Package vehicle implementa las publicaciones de vehículos: el catálogo
// público (HU-VIS-01) y la publicación y el reporte de venta por parte del
// vendedor (HU-VEN-01).
package vehicle

import (
	"errors"
	"time"
)

// Status es el estado de una publicación.
//
//	pendiente ──(administrador aprueba)──> publicada
//	pendiente | publicada ──(vendedor reporta la venta)──> vendida
//	cualquier estado ──(administrador elimina)──> (se borra)
//
// Solo las publicaciones en estado publicada aparecen en el catálogo.
type Status string

const (
	StatusPendiente Status = "pendiente"
	StatusPublicada Status = "publicada"
	StatusVendida   Status = "vendida"
)

// Vehicle es una publicación de un vehículo en venta.
type Vehicle struct {
	ID          string    `json:"id"`
	VendedorID  string    `json:"vendedor_id"`
	Marca       string    `json:"marca"`
	Modelo      string    `json:"modelo"`
	Anio        int       `json:"anio"`
	Precio      int64     `json:"precio"`
	Kilometraje int       `json:"kilometraje"`
	Estado      Status    `json:"estado"`
	CreadoEn    time.Time `json:"creado_en"`
}

var (
	ErrNotFound    = errors.New("vehículo no encontrado")
	ErrNotOwner    = errors.New("el vehículo pertenece a otro vendedor")
	ErrAlreadySold = errors.New("el vehículo ya fue reportado como vendido")
	ErrNotPending  = errors.New("solo se pueden aprobar publicaciones pendientes")
)

// Repository es el contrato de persistencia de vehículos.
type Repository interface {
	// Create guarda una publicación nueva.
	Create(v Vehicle) error
	// FindByID devuelve ErrNotFound si no existe.
	FindByID(id string) (Vehicle, error)
	// List devuelve todas las publicaciones, de la más reciente a la más antigua.
	List() ([]Vehicle, error)
	// Update aplica change a la publicación bajo bloqueo y la guarda solo si
	// change no devuelve error. Así la verificación (dueño, estado) y el
	// cambio ocurren juntos, sin que otra solicitud se cuele en medio.
	// Devuelve ErrNotFound si no existe.
	Update(id string, change func(*Vehicle) error) (Vehicle, error)
	// Delete elimina la publicación. Devuelve ErrNotFound si no existe.
	Delete(id string) error
	// DeleteBySeller elimina todas las publicaciones de un vendedor.
	DeleteBySeller(sellerID string) error
}
