package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/testimg"
)

// vehicleForm es una marca para do(): en vez de JSON, envía la publicación
// estándar (Renault Koleos) como multipart/form-data con dos fotos.
const vehicleForm = "\x00publicacion-estandar"

func standardFields() map[string]string {
	return map[string]string{"marca": "Renault", "modelo": "Koleos", "anio": "2019", "precio": "78000000", "kilometraje": "60000"}
}

func twoImages() [][]byte { return [][]byte{testimg.PNG(), testimg.PNG()} }

// sendMultipart envía un POST multipart con campos y fotos en "imagenes".
func sendMultipart(r http.Handler, path string, fields map[string]string, images [][]byte, authorization string, session *http.Cookie) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for i, data := range images {
		part, _ := mw.CreateFormFile("imagenes", "foto"+string(rune('a'+i))+".png")
		_, _ = part.Write(data)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if session != nil {
		req.AddCookie(session)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// postPublish envía el formulario de la página /publicar con una foto.
func postPublish(app *testApp, values url.Values, session *http.Cookie) *httptest.ResponseRecorder {
	fields := map[string]string{}
	for k := range values {
		fields[k] = values.Get(k)
	}
	return sendMultipart(app.router, "/publicar", fields, [][]byte{testimg.PNG()}, "", session)
}
