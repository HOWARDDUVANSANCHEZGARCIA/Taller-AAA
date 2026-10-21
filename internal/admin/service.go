// Package admin implementa la moderación del administrador (HU-ADM-01):
// aprobar publicaciones y eliminar publicaciones o usuarios.
package admin

import (
	"errors"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// ErrSelfDelete evita que el administrador borre su propia cuenta y deje el
// sistema sin nadie que pueda moderarlo.
var ErrSelfDelete = errors.New("no puede eliminar su propia cuenta de administrador")

// Publication es una publicación en cualquier estado con el nombre de su vendedor.
type Publication struct {
	vehicle.Vehicle
	Vendedor string
}

type Service struct {
	users    user.Repository
	vehicles *vehicle.Service
}

func NewService(users user.Repository, vehicles *vehicle.Service) *Service {
	return &Service{users: users, vehicles: vehicles}
}

// Publications devuelve todas las publicaciones (pendientes, publicadas y
// vendidas), de la más reciente a la más antigua.
func (s *Service) Publications() ([]Publication, error) {
	all, err := s.vehicles.All()
	if err != nil {
		return nil, err
	}
	pubs := make([]Publication, 0, len(all))
	for _, v := range all {
		seller, err := s.users.FindByID(v.VendedorID)
		if err != nil && !errors.Is(err, user.ErrNotFound) {
			return nil, err
		}
		pubs = append(pubs, Publication{Vehicle: v, Vendedor: seller.Nombre})
	}
	return pubs, nil
}

// Users devuelve todos los usuarios registrados.
func (s *Service) Users() ([]user.User, error) {
	return s.users.List()
}

// Approve pasa una publicación pendiente a publicada.
func (s *Service) Approve(vehicleID string) (vehicle.Vehicle, error) {
	return s.vehicles.Approve(vehicleID)
}

// DeletePublication elimina una publicación en cualquier estado.
func (s *Service) DeletePublication(vehicleID string) error {
	return s.vehicles.Delete(vehicleID)
}

// DeleteUser elimina un usuario y sus publicaciones. Primero se borran las
// publicaciones: si algo falla a mitad, no quedan vehículos de un usuario
// inexistente. Su token deja de servir de inmediato porque cada solicitud
// vuelve a buscar al usuario.
func (s *Service) DeleteUser(adminID, userID string) error {
	if adminID == userID {
		return ErrSelfDelete
	}
	if _, err := s.users.FindByID(userID); err != nil {
		return err
	}
	if err := s.vehicles.DeleteBySeller(userID); err != nil {
		return err
	}
	return s.users.Delete(userID)
}
