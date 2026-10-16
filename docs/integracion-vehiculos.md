# Vehículos, catálogo y sitio web: guía de integración (Integrante 2)

Cubre HU-VIS-01 (catálogo público con contacto del vendedor) y HU-VEN-01
(publicar en estado pendiente y reportar la venta), más las páginas web que
sirve Gin.

## Ejecutar

```bash
go mod tidy                      # primera vez: descarga Gin v1.11.0 y genera go.sum
go test ./...
AUTH_SECRET=cambia-esto go run ./cmd/server
# abrir http://localhost:8080
```

| Variable | Por defecto | Uso |
|---|---|---|
| `VEHICLES_FILE` | `data/vehicles.json` | Archivo de vehículos. |
| `IMAGES_DIR` | `data/imagenes` | Carpeta de fotos (una subcarpeta por vehículo). |

Las demás variables (`AUTH_SECRET`, `DATA_FILE`, `PORT`) están en
[integracion-auth.md](integracion-auth.md).

## Estados de una publicación

```
pendiente ──(administrador aprueba, HU-ADM-01)──> publicada
pendiente | publicada ──(el dueño reporta la venta)──> vendida
```

Solo las publicadas aparecen en el catálogo. El estado y el vendedor los fija
el servidor; si el cliente los envía, se ignoran.

## API JSON

| Método y ruta | Quién | Respuesta |
|---|---|---|
| `GET /vehiculos` | cualquiera | `200` lista de publicados, cada uno con `vendedor` (nombre, email, teléfono) |
| `GET /vehiculos/:id` | cualquiera | `200` el vehículo con `vendedor`; `404` si no existe o no está publicado |
| `POST /vehiculos` | vendedor | `201` publicación en estado `pendiente` (multipart: datos + 1 a 8 fotos) |
| `GET /vehiculos/:id/imagenes/:nombre` | ver abajo | `200` la foto; `404` si no existe o no puede verla |
| `POST /vehiculos/:id/imagenes` | vendedor dueño, solo pendiente | `200` agrega fotos; `409` si ya fue aprobada |
| `DELETE /vehiculos/:id/imagenes/:nombre` | vendedor dueño, solo pendiente | `200` quita una foto (siempre queda al menos una) |
| `POST /vehiculos/:id/venta` | vendedor dueño | `200` estado `vendida`; `403` si es de otro; `409` si ya estaba vendida |
| `GET /vendedor/vehiculos` | vendedor | `200` sus publicaciones en cualquier estado |

```bash
TOKEN=…   # de POST /auth/login
curl -X POST localhost:8080/vehiculos -H "Authorization: Bearer $TOKEN" \
  -F marca=Renault -F modelo=Koleos -F anio=2019 -F precio=78000000 -F kilometraje=60000 \
  -F imagenes=@frente.jpg -F imagenes=@interior.png
```

Errores: `400` datos inválidos, `401` sin token válido, `403` rol o dueño
incorrecto, `404`, `409`, `413` formulario demasiado grande. Siempre con la
forma `{"error":"mensaje"}`.

## Fotos

- **Dónde:** archivos en `data/imagenes/<id del vehículo>/`. En `vehicles.json`
  cada vehículo solo guarda la lista de nombres (`"imagenes": ["a1b2….jpg", …]`);
  la primera es la portada.
- **Validación (`internal/imagestore`, solo librería estándar):**
  - De 1 a 8 fotos por publicación, máximo 5 MB cada una y 6000 × 6000 px.
  - Solo JPG o PNG, comprobado leyendo la cabecera del archivo con
    `image.DecodeConfig`; no se confía en el nombre ni en el `Content-Type` del
    cliente.
  - El cuerpo de la solicitud se corta en 41 MB (`http.MaxBytesReader`).
- **Nombres:** los genera el servidor (32 caracteres aleatorios + extensión) y
  se descarta el nombre original. `Path` solo acepta ese formato, así que no se
  puede leer ni escribir fuera de la carpeta (`../`).
- **Quién las ve (AAA):** las de un vehículo publicado son públicas. Las de uno
  pendiente o vendido solo las ven su vendedor y el administrador; los demás
  reciben `404`, igual que en el catálogo. La ruta acepta el token por
  `Authorization: Bearer` o por la cookie de sesión (`OptionalAuthenticate`),
  porque el navegador las pide desde `<img>`.
- **Cambios:** el vendedor puede agregar y quitar fotos solo mientras la
  publicación está pendiente, para que el administrador apruebe exactamente lo
  que se mostrará.
- **Respuesta:** `X-Content-Type-Options: nosniff`, para que el navegador nunca
  la interprete como otra cosa que una imagen.
- **Accounting:** cada vez que alguien ve una foto queda una entrada
  `GET /vehiculos/:id/imagenes/:nombre` con su resultado.
- **Borrado:** al eliminar una publicación o un usuario se borran también sus fotos.

## Sitio web (plantillas de Gin)

| Página | Quién |
|---|---|
| `/` catálogo y `/catalogo/:id` detalle con contacto | cualquiera |
| `/ingresar`, `/registro`, `POST /salir` | cualquiera |
| `/publicar`, `/mis-vehiculos`, `POST /mis-vehiculos/:id/vendido` | vendedor |
| `/mis-vehiculos/:id/fotos` (agregar y quitar fotos mientras está pendiente) | vendedor dueño |

- Plantillas `html/template` + un CSS, en `internal/httpapi/web/`, embebidos en
  el ejecutable. Sin JavaScript ni librerías externas.
- `html/template` escapa todo lo que escribe el usuario (sin XSS).
- La sesión es el mismo token de `/auth/login` guardado en la cookie `sesion`:
  `HttpOnly` (un script no puede leerla) y `SameSite=Strict` (otro sitio no
  puede enviar formularios a nombre del usuario: CSRF). Sin `Secure` porque
  esta entrega es HTTP.
- Las páginas llaman a los mismos servicios que la API, así que validaciones,
  estados y verificación del dueño son idénticos. La traducción de errores a
  códigos HTTP está en un solo lugar: `errorStatus` en `httpapi/errors.go`.

## Relación con los demás componentes

- **Administración (`internal/admin`):** aprueba con `vehicle.Service.Approve`,
  que verifica y cambia el estado bajo el mismo bloqueo (`Repository.Update`).
  Al eliminar un usuario se eliminan también sus publicaciones.
- **Accounting (`internal/accounting`):** un middleware global en `NewRouter`
  registra cada solicitud de la API y de la web. `CurrentUser(c)` da el actor
  en ambas, y `c.ClientIP()` es confiable porque `SetTrustedProxies(nil)`
  ignora `X-Forwarded-For`.

## Decisiones de diseño

- Datos del vehículo: marca, modelo, año, precio (pesos, entero), kilometraje y de 1 a 8 fotos.
- Galería del detalle sin JavaScript: un radio oculto por foto; miniaturas y
  flechas son `<label>` que lo marcan, y el CSS muestra la foto marcada. La
  vista ampliada se abre con `#ampliada-N` (`:target`) y ofrece "Ajustar a la
  pantalla" y "Ampliar 2×" (otro par de radios). Con el teclado, las flechas
  cambian de foto.
- `jsonfile` comparte la carga y la escritura atómica con el repositorio de
  usuarios; `ids` comparte el generador de IDs aleatorios (no se pueden
  adivinar ni recorrer en orden).
- Un vehículo pendiente o vendido responde `404` en el catálogo, igual que uno
  inexistente, para no revelar publicaciones que no son públicas.
