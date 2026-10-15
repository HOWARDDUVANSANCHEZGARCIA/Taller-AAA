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
| `POST /vehiculos` | vendedor | `201` publicación en estado `pendiente` |
| `POST /vehiculos/:id/venta` | vendedor dueño | `200` estado `vendida`; `403` si es de otro; `409` si ya estaba vendida |
| `GET /vendedor/vehiculos` | vendedor | `200` sus publicaciones en cualquier estado |

```bash
TOKEN=…   # de POST /auth/login
curl -X POST localhost:8080/vehiculos -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"marca":"Renault","modelo":"Koleos","anio":2019,"precio":78000000,"kilometraje":60000}'
```

Errores: `400` datos inválidos, `401` sin token válido, `403` rol o dueño
incorrecto, `404`, `409`. Siempre con la forma `{"error":"mensaje"}`.

## Sitio web (plantillas de Gin)

| Página | Quién |
|---|---|
| `/` catálogo y `/catalogo/:id` detalle con contacto | cualquiera |
| `/ingresar`, `/registro`, `POST /salir` | cualquiera |
| `/publicar`, `/mis-vehiculos`, `POST /mis-vehiculos/:id/vendido` | vendedor |

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

## Qué necesita cada integrante

- **Integrante 3 (autorización y administración):**
  - `RequireRole(roles ...user.Role)` ya está en `httpapi/middleware.go`
    (403 si el rol no alcanza). Úselo después de `Authenticate`.
  - Para aprobar use `vehicle.Repository.Update`, que hace la verificación y
    el cambio bajo el mismo bloqueo:
    ```go
    repo.Update(id, func(v *vehicle.Vehicle) error {
        if v.Estado != vehicle.StatusPendiente { return ErrNoPendiente }
        v.Estado = vehicle.StatusPublicada
        return nil
    })
    ```
  - Para eliminar publicaciones agregue `Delete(id string) error` a
    `vehicle.Repository` y `FileRepository` (bloquear, quitar, `jsonfile.Save`).
  - Si elimina un usuario, sus vehículos dejan de verse en el catálogo
    automáticamente (no hay contacto que mostrar). Decidan si además se borran.
  - Agregue sus errores nuevos a `errorStatus`.
- **Integrante 4 (accounting):** un middleware global en `NewRouter`
  (`httpapi/router.go`) ve todas las rutas, API y web. Después de `c.Next()`,
  `CurrentUser(c)` devuelve al usuario en ambas (la web usa la misma clave de
  contexto). `c.ClientIP()` es confiable porque `SetTrustedProxies(nil)`
  ignora `X-Forwarded-For`.

## Decisiones de diseño

- Datos del vehículo: marca, modelo, año, precio (pesos, entero) y kilometraje.
- `jsonfile` comparte la carga y la escritura atómica con el repositorio de
  usuarios; `ids` comparte el generador de IDs aleatorios (no se pueden
  adivinar ni recorrer en orden).
- Un vehículo pendiente o vendido responde `404` en el catálogo, igual que uno
  inexistente, para no revelar publicaciones que no son públicas.
