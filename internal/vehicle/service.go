package vehicle

import (
	"errors"
	"slices"
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

	MinImages = 1
	MaxImages = 8
)

// SellerDirectory es lo único que el catálogo necesita de los usuarios:
// encontrar al vendedor de una publicación. user.Repository lo cumple.
type SellerDirectory interface {
	FindByID(id string) (user.User, error)
}

// ImageStore guarda las fotos de cada vehículo. imagestore.Store lo cumple.
type ImageStore interface {
	// Save valida y guarda una imagen; devuelve el nombre generado.
	Save(vehicleID string, data []byte) (string, error)
	// Path devuelve la ruta del archivo de una imagen guardada.
	Path(vehicleID, name string) (string, error)
	// Delete borra una imagen del vehículo.
	Delete(vehicleID, name string) error
	// DeleteAll borra todas las imágenes del vehículo.
	DeleteAll(vehicleID string) error
}

// PublishInput son los datos que envía el vendedor al publicar.
type PublishInput struct {
	Marca       string
	Modelo      string
	Anio        int
	Precio      int64
	Kilometraje int
	Imagenes    [][]byte // contenido de cada foto, en el orden en que se muestran
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
	images  ImageStore
	now     func() time.Time
}

func NewService(repo Repository, sellers SellerDirectory, images ImageStore) *Service {
	return &Service{repo: repo, sellers: sellers, images: images, now: time.Now}
}

// Publish registra la publicación en estado pendiente y la asocia al
// vendedor autenticado. El vendedor y el estado los fija el servidor. Si algo
// falla, no quedan imágenes sueltas en disco.
func (s *Service) Publish(sellerID string, in PublishInput) (v Vehicle, err error) {
	in.Marca = strings.TrimSpace(in.Marca)
	in.Modelo = strings.TrimSpace(in.Modelo)
	if err := s.validate(in); err != nil {
		return Vehicle{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Vehicle{}, err
	}
	defer func() {
		if err != nil {
			_ = s.images.DeleteAll(id)
		}
	}()
	names := make([]string, 0, len(in.Imagenes))
	for _, data := range in.Imagenes {
		name, err := s.images.Save(id, data)
		if err != nil {
			return Vehicle{}, err
		}
		names = append(names, name)
	}
	v = Vehicle{
		ID:          id,
		VendedorID:  sellerID,
		Marca:       in.Marca,
		Modelo:      in.Modelo,
		Anio:        in.Anio,
		Precio:      in.Precio,
		Kilometraje: in.Kilometraje,
		Estado:      StatusPendiente,
		Imagenes:    names,
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

// Approve publica una publicación pendiente (HU-ADM-01): pasa a publicada y
// desde ese momento aparece en el catálogo.
func (s *Service) Approve(vehicleID string) (Vehicle, error) {
	return s.repo.Update(vehicleID, func(v *Vehicle) error {
		if v.Estado != StatusPendiente {
			return ErrNotPending
		}
		v.Estado = StatusPublicada
		return nil
	})
}

// Delete elimina una publicación en cualquier estado, con sus imágenes (lo
// usa el administrador).
func (s *Service) Delete(vehicleID string) error {
	if err := s.repo.Delete(vehicleID); err != nil {
		return err
	}
	return s.images.DeleteAll(vehicleID)
}

// DeleteBySeller elimina todas las publicaciones de un vendedor, con sus imágenes.
func (s *Service) DeleteBySeller(sellerID string) error {
	own, err := s.BySeller(sellerID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteBySeller(sellerID); err != nil {
		return err
	}
	for _, v := range own {
		if err := s.images.DeleteAll(v.ID); err != nil {
			return err
		}
	}
	return nil
}

// Own devuelve una publicación del vendedor; ErrNotOwner si es de otro.
func (s *Service) Own(sellerID, vehicleID string) (Vehicle, error) {
	v, err := s.repo.FindByID(vehicleID)
	if err != nil {
		return Vehicle{}, err
	}
	if v.VendedorID != sellerID {
		return Vehicle{}, ErrNotOwner
	}
	return v, nil
}

// AddImages agrega fotos a una publicación propia que sigue pendiente. Después
// de la aprobación ya no se puede cambiar, para que el administrador apruebe
// exactamente lo que se va a mostrar.
//
// Las fotos se guardan antes de tomar el bloqueo del repositorio (escribir
// archivos es lento) y la regla se vuelve a comprobar dentro de Update; si
// ya no se cumple, se borran los archivos recién guardados.
func (s *Service) AddImages(sellerID, vehicleID string, images [][]byte) (Vehicle, error) {
	v, err := s.repo.FindByID(vehicleID)
	if err != nil {
		return Vehicle{}, err
	}
	if err := checkEditable(sellerID, v, len(v.Imagenes)+len(images)); err != nil {
		return Vehicle{}, err
	}
	var saved []string
	for _, data := range images {
		name, err := s.images.Save(vehicleID, data)
		if err != nil {
			s.deleteImages(vehicleID, saved)
			return Vehicle{}, err
		}
		saved = append(saved, name)
	}
	updated, err := s.repo.Update(vehicleID, func(v *Vehicle) error {
		if err := checkEditable(sellerID, *v, len(v.Imagenes)+len(saved)); err != nil {
			return err
		}
		v.Imagenes = append(slices.Clone(v.Imagenes), saved...)
		return nil
	})
	if err != nil {
		s.deleteImages(vehicleID, saved)
	}
	return updated, err
}

// RemoveImage quita una foto de una publicación propia pendiente. Siempre
// debe quedar al menos una.
func (s *Service) RemoveImage(sellerID, vehicleID, name string) (Vehicle, error) {
	updated, err := s.repo.Update(vehicleID, func(v *Vehicle) error {
		if err := checkEditable(sellerID, *v, len(v.Imagenes)-1); err != nil {
			return err
		}
		i := slices.Index(v.Imagenes, name)
		if i < 0 {
			return ErrNotFound
		}
		v.Imagenes = slices.Delete(slices.Clone(v.Imagenes), i, i+1)
		return nil
	})
	if err != nil {
		return Vehicle{}, err
	}
	return updated, s.images.Delete(vehicleID, name)
}

// checkEditable aplica las reglas para cambiar fotos: ser el dueño, que siga
// pendiente y que el total quede entre MinImages y MaxImages.
func checkEditable(sellerID string, v Vehicle, total int) error {
	switch {
	case v.VendedorID != sellerID:
		return ErrNotOwner
	case v.Estado != StatusPendiente:
		return ErrNotEditable
	case total < MinImages:
		return &apperr.ValidationError{Msg: "la publicación debe conservar al menos una foto"}
	case total > MaxImages:
		return &apperr.ValidationError{Msg: "una publicación puede tener máximo 8 fotos"}
	}
	return nil
}

func (s *Service) deleteImages(vehicleID string, names []string) {
	for _, name := range names {
		_ = s.images.Delete(vehicleID, name)
	}
}

// ImagePath devuelve el archivo de una foto si el solicitante puede verla.
// Las fotos de un vehículo publicado son públicas; las de uno pendiente o
// vendido solo las ven su vendedor y el administrador. A los demás se les
// responde ErrNotFound, igual que en el catálogo, para no revelar nada.
// viewer es nil si el solicitante es un visitante.
func (s *Service) ImagePath(viewer *user.User, vehicleID, name string) (string, error) {
	v, err := s.repo.FindByID(vehicleID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(v.Imagenes, name) {
		return "", ErrNotFound
	}
	if v.Estado != StatusPublicada && !canSeeUnpublished(viewer, v) {
		return "", ErrNotFound
	}
	return s.images.Path(vehicleID, name)
}

func canSeeUnpublished(viewer *user.User, v Vehicle) bool {
	return viewer != nil && (viewer.ID == v.VendedorID || viewer.HasRole(user.RoleAdministrador))
}

// All devuelve todas las publicaciones en cualquier estado, de la más
// reciente a la más antigua (para el administrador).
func (s *Service) All() ([]Vehicle, error) {
	return s.repo.List()
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
	if n := len(in.Imagenes); n < MinImages || n > MaxImages {
		return &apperr.ValidationError{Msg: "debe subir entre 1 y 8 fotos del vehículo"}
	}
	return nil
}
