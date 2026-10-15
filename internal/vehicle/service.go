package vehicle

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/ids"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

const (
	maxTextLen     = 50
	minAnio        = 1900
	maxPrecio      = 10_000_000_000 // diez mil millones de pesos
	maxKilometraje = 2_000_000
)

// SellerDirectory es lo único que el catálogo necesita de los usuarios:
// encontrar al vendedor de una publicación. user.Repository lo cumple.
type SellerDirectory interface {
	FindByID(id string) (user.User, error)
}

// PublishInput son los datos que envía el vendedor al publicar.
type PublishInput struct {
	Marca       string
	Modelo      string
	Anio        int
	Precio      int64
	Kilometraje int
}

// Contact son los datos del vendedor que se muestran al visitante.
type Contact struct {
	Nombre   string
	Email    string
	Telefono string
}

// Listing es un vehículo del catálogo público junto con el contacto de su vendedor.
type Listing struct {
	Vehicle
	Vendedor Contact
}

type Service struct {
	repo    Repository
	sellers SellerDirectory
	now     func() time.Time
}

func NewService(repo Repository, sellers SellerDirectory) *Service {
	return &Service{repo: repo, sellers: sellers, now: time.Now}
}

// Publish registra la publicación en estado pendiente y la asocia al
// vendedor autenticado. El vendedor y el estado los fija el servidor.
func (s *Service) Publish(sellerID string, in PublishInput) (Vehicle, error) {
	in.Marca = strings.TrimSpace(in.Marca)
	in.Modelo = strings.TrimSpace(in.Modelo)
	if err := s.validate(in); err != nil {
		return Vehicle{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Vehicle{}, err
	}
	v := Vehicle{
		ID:          id,
		VendedorID:  sellerID,
		Marca:       in.Marca,
		Modelo:      in.Modelo,
		Anio:        in.Anio,
		Precio:      in.Precio,
		Kilometraje: in.Kilometraje,
		Estado:      StatusPendiente,
		CreadoEn:    s.now().UTC(),
	}
	if err := s.repo.Create(v); err != nil {
		return Vehicle{}, err
	}
	return v, nil
}

// ReportSale marca el vehículo como vendido, con lo que sale del catálogo.
// Solo el vendedor dueño de la publicación puede hacerlo.
func (s *Service) ReportSale(sellerID, vehicleID string) (Vehicle, error) {
	return s.repo.Update(vehicleID, func(v *Vehicle) error {
		if v.VendedorID != sellerID {
			return ErrNotOwner
		}
		if v.Estado == StatusVendida {
			return ErrAlreadySold
		}
		v.Estado = StatusVendida
		return nil
	})
}

// BySeller devuelve las publicaciones del vendedor en cualquier estado, para
// que pueda ver si siguen pendientes y reportar sus ventas.
func (s *Service) BySeller(sellerID string) ([]Vehicle, error) {
	all, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	var own []Vehicle
	for _, v := range all {
		if v.VendedorID == sellerID {
			own = append(own, v)
		}
	}
	return own, nil
}

// Catalog devuelve los vehículos publicados con el contacto de su vendedor.
func (s *Service) Catalog() ([]Listing, error) {
	all, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	var listings []Listing
	for _, v := range all {
		if v.Estado != StatusPublicada {
			continue
		}
		l, err := s.withContact(v)
		if errors.Is(err, ErrNotFound) {
			continue // el vendedor ya no existe: no hay a quién contactar
		}
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, nil
}

// CatalogItem devuelve un vehículo del catálogo con el contacto de su
// vendedor. Si no está publicado responde ErrNotFound, igual que si no
// existiera, para no revelar publicaciones pendientes o vendidas.
func (s *Service) CatalogItem(id string) (Listing, error) {
	v, err := s.repo.FindByID(id)
	if err != nil {
		return Listing{}, err
	}
	if v.Estado != StatusPublicada {
		return Listing{}, ErrNotFound
	}
	return s.withContact(v)
}

func (s *Service) withContact(v Vehicle) (Listing, error) {
	seller, err := s.sellers.FindByID(v.VendedorID)
	if errors.Is(err, user.ErrNotFound) {
		return Listing{}, ErrNotFound
	}
	if err != nil {
		return Listing{}, err
	}
	return Listing{
		Vehicle:  v,
		Vendedor: Contact{Nombre: seller.Nombre, Email: seller.Email, Telefono: seller.Telefono},
	}, nil
}

func (s *Service) validate(in PublishInput) error {
	if n := utf8.RuneCountInString(in.Marca); n == 0 || n > maxTextLen {
		return &apperr.ValidationError{Msg: "la marca es obligatoria y debe tener máximo 50 caracteres"}
	}
	if n := utf8.RuneCountInString(in.Modelo); n == 0 || n > maxTextLen {
		return &apperr.ValidationError{Msg: "el modelo es obligatorio y debe tener máximo 50 caracteres"}
	}
	if maxAnio := s.now().Year() + 1; in.Anio < minAnio || in.Anio > maxAnio {
		return &apperr.ValidationError{Msg: "el año no es válido"}
	}
	if in.Precio <= 0 || in.Precio > maxPrecio {
		return &apperr.ValidationError{Msg: "el precio debe ser mayor que cero y máximo 10.000.000.000"}
	}
	if in.Kilometraje < 0 || in.Kilometraje > maxKilometraje {
		return &apperr.ValidationError{Msg: "el kilometraje debe estar entre 0 y 2.000.000"}
	}
	return nil
}
