package httpapi

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/apperr"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/imagestore"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

// maxUploadBody es lo máximo que se lee del cuerpo al publicar: todas las
// fotos permitidas más un margen para los campos de texto. Lo que pase de
// ahí se corta sin leerlo entero.
const maxUploadBody = vehicle.MaxImages*imagestore.MaxBytes + 1<<20

var errTooLarge = errors.New("el formulario supera el tamaño máximo: hasta 8 fotos de 5 MB cada una")

// publishForm guarda lo escrito tal cual, para devolverlo si hay un error.
type publishForm struct{ Marca, Modelo, Anio, Precio, Kilometraje string }

// readPublishForm lee la publicación enviada como multipart/form-data, tanto
// desde la API como desde la página: campos marca, modelo, anio, precio y
// kilometraje, y una o más fotos en el campo "imagenes".
func readPublishForm(c *gin.Context) (publishForm, vehicle.PublishInput, error) {
	mf, err := parseMultipart(c)
	if err != nil {
		return publishForm{}, vehicle.PublishInput{}, err
	}
	form := publishForm{
		Marca:       c.PostForm("marca"),
		Modelo:      c.PostForm("modelo"),
		Anio:        c.PostForm("anio"),
		Precio:      c.PostForm("precio"),
		Kilometraje: c.PostForm("kilometraje"),
	}
	in, err := form.toInput()
	if err != nil {
		return form, in, err
	}
	in.Imagenes, err = uploadedImages(mf)
	return form, in, err
}

// readImages lee solo las fotos del campo "imagenes" (para agregar fotos a
// una publicación pendiente).
func readImages(c *gin.Context) ([][]byte, error) {
	mf, err := parseMultipart(c)
	if err != nil {
		return nil, err
	}
	return uploadedImages(mf)
}

// parseMultipart limita el tamaño del cuerpo y lo interpreta como
// multipart/form-data.
func parseMultipart(c *gin.Context) (*multipart.Form, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBody)
	mf, err := c.MultipartForm()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errTooLarge
		}
		return nil, &apperr.ValidationError{Msg: "envíe el formulario como multipart/form-data con las fotos en el campo imagenes"}
	}
	return mf, nil
}

func uploadedImages(mf *multipart.Form) ([][]byte, error) {
	var images [][]byte
	for _, fh := range mf.File["imagenes"] {
		data, err := readUpload(fh)
		if err != nil {
			return nil, err
		}
		images = append(images, data)
	}
	return images, nil
}

func readUpload(fh *multipart.FileHeader) ([]byte, error) {
	if fh.Size > imagestore.MaxBytes {
		return nil, &apperr.ValidationError{Msg: "cada imagen debe pesar máximo 5 MB"}
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, imagestore.MaxBytes+1))
}

// toInput convierte los textos del formulario en números. Acepta puntos y
// espacios como separadores de miles ("78.000.000").
func (f publishForm) toInput() (vehicle.PublishInput, error) {
	anio, err := strconv.Atoi(strings.TrimSpace(f.Anio))
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el año debe ser un número, por ejemplo 2019"}
	}
	precio, err := strconv.ParseInt(stripThousands(f.Precio), 10, 64)
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el precio debe ser un número entero de pesos, por ejemplo 78.000.000"}
	}
	km, err := strconv.Atoi(stripThousands(f.Kilometraje))
	if err != nil {
		return vehicle.PublishInput{}, &apperr.ValidationError{Msg: "el kilometraje debe ser un número entero, por ejemplo 60.000"}
	}
	return vehicle.PublishInput{Marca: f.Marca, Modelo: f.Modelo, Anio: anio, Precio: precio, Kilometraje: km}, nil
}

// stripThousands quita los separadores de miles; cualquier otro carácter se deja
// para que strconv lo rechace.
func stripThousands(s string) string {
	return strings.NewReplacer(".", "", " ", "").Replace(strings.TrimSpace(s))
}

// imageURL es la dirección pública de una foto de un vehículo.
func imageURL(vehicleID, name string) string {
	return "/vehiculos/" + vehicleID + "/imagenes/" + name
}
