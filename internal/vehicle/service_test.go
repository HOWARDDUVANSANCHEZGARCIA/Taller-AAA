package vehicle

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/imagestore"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/testimg"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

type fixture struct {
	svc    *Service
	imgDir string
	repo   *FileRepository
	users  *user.FileRepository
	seller user.User
	other  user.User
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	users, err := user.NewFileRepository(filepath.Join(dir, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewFileRepository(filepath.Join(dir, "vehicles.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{
		svc:    NewService(repo, users, imagestore.New(filepath.Join(dir, "imagenes"))),
		imgDir: filepath.Join(dir, "imagenes"),
		repo:   repo,
		users:  users,
		seller: user.User{ID: "v1", Nombre: "Ana", Email: "ana@correo.com", Telefono: "3001234567", Rol: user.RoleVendedor},
		other:  user.User{ID: "v2", Nombre: "Beto", Email: "beto@correo.com", Telefono: "3007654321", Rol: user.RoleVendedor},
	}
	for _, u := range []user.User{f.seller, f.other} {
		if err := users.Create(u); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func validPublish() PublishInput {
	return PublishInput{Marca: " Renault ", Modelo: "Koleos", Anio: 2019, Precio: 78_000_000, Kilometraje: 60_000,
		Imagenes: [][]byte{testimg.PNG(), testimg.PNG()}}
}

// approve hace lo que hará el administrador (HU-ADM-01) para poder probar el catálogo.
func (f fixture) approve(t *testing.T, id string) {
	t.Helper()
	if _, err := f.repo.Update(id, func(v *Vehicle) error { v.Estado = StatusPublicada; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestPublishCreatesPendingVehicleOfSeller(t *testing.T) {
	f := newFixture(t)

	v, err := f.svc.Publish(f.seller.ID, validPublish())
	if err != nil {
		t.Fatal(err)
	}
	if v.Estado != StatusPendiente {
		t.Errorf("Estado = %q; quería %q", v.Estado, StatusPendiente)
	}
	if v.VendedorID != f.seller.ID {
		t.Errorf("VendedorID = %q; quería %q", v.VendedorID, f.seller.ID)
	}
	if v.Marca != "Renault" {
		t.Errorf("Marca = %q; debe quitar espacios", v.Marca)
	}
	if stored, err := f.repo.FindByID(v.ID); err != nil || !reflect.DeepEqual(stored, v) {
		t.Errorf("guardado = %+v, %v", stored, err)
	}
	if len(v.Imagenes) != 2 {
		t.Fatalf("Imagenes = %v; quería 2", v.Imagenes)
	}
	for _, name := range v.Imagenes {
		if _, err := os.Stat(filepath.Join(f.imgDir, v.ID, name)); err != nil {
			t.Errorf("la imagen %s debe existir en disco: %v", name, err)
		}
	}
}

// Las fotos siguen la misma regla del catálogo: públicas solo si la
// publicación está publicada; si no, solo para su vendedor y el administrador.
func TestImagePathAccess(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish())
	name := v.Imagenes[0]
	admin := &user.User{ID: "admin", Rol: user.RoleAdministrador}

	check := func(label string, viewer *user.User, wantOK bool) {
		t.Helper()
		path, err := f.svc.ImagePath(viewer, v.ID, name)
		if wantOK && (err != nil || path == "") {
			t.Errorf("%s: debe poder ver la foto: %v", label, err)
		}
		if !wantOK && !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v; quería ErrNotFound", label, err)
		}
	}

	check("visitante, pendiente", nil, false)
	check("otro vendedor, pendiente", &f.other, false)
	check("dueño, pendiente", &f.seller, true)
	check("administrador, pendiente", admin, true)

	f.approve(t, v.ID)
	check("visitante, publicada", nil, true)

	_, _ = f.svc.ReportSale(f.seller.ID, v.ID)
	check("visitante, vendida", nil, false)
	check("dueño, vendida", &f.seller, true)

	if _, err := f.svc.ImagePath(&f.seller, v.ID, "../../users.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nombre ajeno a la publicación: err = %v", err)
	}
	if _, err := f.svc.ImagePath(&f.seller, "no-existe", name); !errors.Is(err, ErrNotFound) {
		t.Errorf("vehículo inexistente: err = %v", err)
	}
}

func TestPublishValidatesInput(t *testing.T) {
	f := newFixture(t)
	nextYear := time.Now().Year() + 1
	mutate := map[string]func(*PublishInput){
		"marca vacía":           func(in *PublishInput) { in.Marca = "  " },
		"modelo vacío":          func(in *PublishInput) { in.Modelo = "" },
		"marca muy larga":       func(in *PublishInput) { in.Marca = string(make([]rune, 51)) },
		"año muy antiguo":       func(in *PublishInput) { in.Anio = 1899 },
		"año futuro":            func(in *PublishInput) { in.Anio = nextYear + 1 },
		"precio cero":           func(in *PublishInput) { in.Precio = 0 },
		"precio negativo":       func(in *PublishInput) { in.Precio = -5 },
		"precio absurdo":        func(in *PublishInput) { in.Precio = maxPrecio + 1 },
		"kilometraje negativo":  func(in *PublishInput) { in.Kilometraje = -1 },
		"sin fotos":             func(in *PublishInput) { in.Imagenes = nil },
		"más de 8 fotos":        func(in *PublishInput) { in.Imagenes = make([][]byte, 9) },
		"foto que no es imagen": func(in *PublishInput) { in.Imagenes = [][]byte{testimg.PNG(), []byte("hola")} },
	}
	for name, change := range mutate {
		in := validPublish()
		change(&in)
		var verr *apperr.ValidationError
		if _, err := f.svc.Publish(f.seller.ID, in); !errors.As(err, &verr) {
			t.Errorf("%s: err = %v; quería ValidationError", name, err)
		}
	}
	if all, _ := f.repo.List(); len(all) != 0 {
		t.Errorf("no debe guardar publicaciones inválidas; hay %d", len(all))
	}
	if entries, _ := os.ReadDir(f.imgDir); len(entries) != 0 {
		t.Errorf("no deben quedar imágenes de publicaciones rechazadas: %v", entries)
	}

	in := validPublish()
	in.Anio = nextYear // el modelo del año siguiente sí es válido
	if _, err := f.svc.Publish(f.seller.ID, in); err != nil {
		t.Errorf("año siguiente: %v", err)
	}
}

func TestCatalogShowsOnlyPublishedWithSellerContact(t *testing.T) {
	f := newFixture(t)
	pending, _ := f.svc.Publish(f.seller.ID, validPublish())
	published, _ := f.svc.Publish(f.seller.ID, validPublish())
	f.approve(t, published.ID)

	list, err := f.svc.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != published.ID {
		t.Fatalf("catálogo = %+v; quería solo la publicada", list)
	}
	want := Contact{Nombre: "Ana", Email: "ana@correo.com", Telefono: "3001234567"}
	if list[0].Vendedor != want {
		t.Errorf("contacto = %+v; quería %+v", list[0].Vendedor, want)
	}

	if _, err := f.svc.CatalogItem(pending.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("detalle de pendiente: err = %v; quería ErrNotFound", err)
	}
	item, err := f.svc.CatalogItem(published.ID)
	if err != nil || item.Vendedor != want {
		t.Errorf("detalle = %+v, %v", item, err)
	}
	if _, err := f.svc.CatalogItem("no-existe"); !errors.Is(err, ErrNotFound) {
		t.Errorf("detalle inexistente: err = %v", err)
	}
}

// Si el vendedor ya no existe (p. ej. lo eliminó el administrador), su
// vehículo no se muestra porque no hay contacto que ofrecer.
func TestCatalogSkipsVehiclesWithoutSeller(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish("vendedor-eliminado", validPublish())
	f.approve(t, v.ID)

	if list, err := f.svc.Catalog(); err != nil || len(list) != 0 {
		t.Errorf("catálogo = %+v, %v; quería vacío", list, err)
	}
	if _, err := f.svc.CatalogItem(v.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("detalle: err = %v; quería ErrNotFound", err)
	}
}

func TestReportSale(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish())
	f.approve(t, v.ID)

	if _, err := f.svc.ReportSale(f.other.ID, v.ID); !errors.Is(err, ErrNotOwner) {
		t.Errorf("otro vendedor: err = %v; quería ErrNotOwner", err)
	}
	if stored, _ := f.repo.FindByID(v.ID); stored.Estado != StatusPublicada {
		t.Error("el intento de otro vendedor no debe cambiar el estado")
	}

	sold, err := f.svc.ReportSale(f.seller.ID, v.ID)
	if err != nil || sold.Estado != StatusVendida {
		t.Fatalf("ReportSale = %+v, %v", sold, err)
	}
	if list, _ := f.svc.Catalog(); len(list) != 0 {
		t.Error("el vehículo vendido debe salir del catálogo")
	}
	if _, err := f.svc.ReportSale(f.seller.ID, v.ID); !errors.Is(err, ErrAlreadySold) {
		t.Errorf("segunda vez: err = %v; quería ErrAlreadySold", err)
	}
	if _, err := f.svc.ReportSale(f.seller.ID, "no-existe"); !errors.Is(err, ErrNotFound) {
		t.Errorf("inexistente: err = %v; quería ErrNotFound", err)
	}
}

func TestReportSaleOfPendingVehicle(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish())

	if sold, err := f.svc.ReportSale(f.seller.ID, v.ID); err != nil || sold.Estado != StatusVendida {
		t.Errorf("vender antes de la aprobación = %+v, %v", sold, err)
	}
}

func TestBySellerReturnsOnlyOwnVehicles(t *testing.T) {
	f := newFixture(t)
	first, _ := f.svc.Publish(f.seller.ID, validPublish())
	f.svc.Publish(f.other.ID, validPublish())
	second, _ := f.svc.Publish(f.seller.ID, validPublish())

	own, err := f.svc.BySeller(f.seller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != 2 || own[0].ID != second.ID || own[1].ID != first.ID {
		t.Errorf("BySeller = %+v; quería las dos de Ana, la más reciente primero", own)
	}
}

func TestApprove(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish())

	approved, err := f.svc.Approve(v.ID)
	if err != nil || approved.Estado != StatusPublicada {
		t.Fatalf("Approve = %+v, %v", approved, err)
	}
	if list, _ := f.svc.Catalog(); len(list) != 1 {
		t.Error("la aprobada debe aparecer en el catálogo")
	}
	if _, err := f.svc.Approve(v.ID); !errors.Is(err, ErrNotPending) {
		t.Errorf("aprobar dos veces: err = %v; quería ErrNotPending", err)
	}
	_, _ = f.svc.ReportSale(f.seller.ID, v.ID)
	if _, err := f.svc.Approve(v.ID); !errors.Is(err, ErrNotPending) {
		t.Errorf("aprobar vendida: err = %v; quería ErrNotPending", err)
	}
	if _, err := f.svc.Approve("no-existe"); !errors.Is(err, ErrNotFound) {
		t.Errorf("inexistente: err = %v", err)
	}
}

func TestDeleteAndDeleteBySeller(t *testing.T) {
	f := newFixture(t)
	a1, _ := f.svc.Publish(f.seller.ID, validPublish())
	f.svc.Publish(f.seller.ID, validPublish())
	b1, _ := f.svc.Publish(f.other.ID, validPublish())

	if err := f.svc.Delete(a1.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(a1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("borrar dos veces: err = %v", err)
	}
	if err := f.svc.DeleteBySeller(f.seller.ID); err != nil {
		t.Fatal(err)
	}
	all, _ := f.svc.All()
	if len(all) != 1 || all[0].ID != b1.ID {
		t.Errorf("quedan %+v; quería solo la de Beto", all)
	}
	if entries, _ := os.ReadDir(f.imgDir); len(entries) != 1 || entries[0].Name() != b1.ID {
		t.Errorf("solo deben quedar las imágenes de Beto: %v", entries)
	}
}

func TestAddAndRemoveImagesWhilePending(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish()) // 2 fotos

	v, err := f.svc.AddImages(f.seller.ID, v.ID, [][]byte{testimg.PNG(), testimg.PNG()})
	if err != nil || len(v.Imagenes) != 4 {
		t.Fatalf("AddImages = %v, %v; quería 4 fotos", v.Imagenes, err)
	}
	if _, err := f.svc.AddImages(f.seller.ID, v.ID, make([][]byte, 5)); err == nil {
		t.Error("no debe pasar de 8 fotos")
	}
	if _, err := f.svc.AddImages(f.other.ID, v.ID, [][]byte{testimg.PNG()}); !errors.Is(err, ErrNotOwner) {
		t.Errorf("otro vendedor: err = %v; quería ErrNotOwner", err)
	}
	var verr *apperr.ValidationError
	if _, err := f.svc.AddImages(f.seller.ID, v.ID, [][]byte{[]byte("no es imagen")}); !errors.As(err, &verr) {
		t.Errorf("archivo inválido: err = %v", err)
	}

	removed := v.Imagenes[0]
	v, err = f.svc.RemoveImage(f.seller.ID, v.ID, removed)
	if err != nil || len(v.Imagenes) != 3 || slices.Contains(v.Imagenes, removed) {
		t.Fatalf("RemoveImage = %v, %v", v.Imagenes, err)
	}
	if _, err := os.Stat(filepath.Join(f.imgDir, v.ID, removed)); !os.IsNotExist(err) {
		t.Error("la foto quitada debe borrarse del disco")
	}
	if entries, _ := os.ReadDir(filepath.Join(f.imgDir, v.ID)); len(entries) != 3 {
		t.Errorf("en disco deben quedar exactamente 3 fotos; hay %d (no deben quedar sueltas)", len(entries))
	}
	if _, err := f.svc.RemoveImage(f.seller.ID, v.ID, "no-existe.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("foto inexistente: err = %v", err)
	}

	for len(v.Imagenes) > 1 {
		v, _ = f.svc.RemoveImage(f.seller.ID, v.ID, v.Imagenes[0])
	}
	if _, err := f.svc.RemoveImage(f.seller.ID, v.ID, v.Imagenes[0]); !errors.As(err, &verr) {
		t.Errorf("quitar la última foto: err = %v; quería ValidationError", err)
	}
}

// Después de aprobada, el administrador ya revisó las fotos: no se pueden cambiar.
func TestImagesLockedAfterApproval(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Publish(f.seller.ID, validPublish())
	f.approve(t, v.ID)

	if _, err := f.svc.AddImages(f.seller.ID, v.ID, [][]byte{testimg.PNG()}); !errors.Is(err, ErrNotEditable) {
		t.Errorf("agregar tras aprobar: err = %v; quería ErrNotEditable", err)
	}
	if _, err := f.svc.RemoveImage(f.seller.ID, v.ID, v.Imagenes[0]); !errors.Is(err, ErrNotEditable) {
		t.Errorf("quitar tras aprobar: err = %v; quería ErrNotEditable", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.imgDir, v.ID)); len(entries) != 2 {
		t.Errorf("no deben quedar archivos nuevos en disco; hay %d", len(entries))
	}
}
