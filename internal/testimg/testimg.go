// Package testimg genera imágenes pequeñas para las pruebas.
package testimg

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// PNG devuelve un PNG válido de 4 × 3 píxeles.
func PNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
