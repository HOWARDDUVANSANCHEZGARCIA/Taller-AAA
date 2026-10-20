# Autorización y administración: guía de integración

Cubre HU-ADM-01 (el administrador aprueba publicaciones y elimina
publicaciones o usuarios) y la parte *Authorization* de RF-01.

## Ejecutar

```bash
go mod tidy                      # primera vez
go test ./...
AUTH_SECRET=clave-tokens ACCOUNTING_SECRET=clave-accounting \
ADMIN_EMAIL=admin@automarket.com ADMIN_PASSWORD=clave-admin-123 \
go run ./cmd/server
```

| Variable | Obligatoria | Por defecto | Uso |
|---|---|---|---|
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | para moderar | — | Crean el administrador al arrancar si no existe (`auth.Service.EnsureAdmin`). |
| `ADMIN_NOMBRE` | no | `Administrador` | Nombre del administrador. |

Las demás variables están en [guia-ejecucion.md](guia-ejecucion.md).

## Roles

| Rol | Cómo se obtiene | Qué puede hacer |
|---|---|---|
| visitante | no presenta un token válido (no es un rol guardado) | ver el catálogo, el detalle con el contacto y las fotos de vehículos publicados; registrarse e ingresar |
| vendedor | registro público (`POST /auth/register`) | publicar, ver sus publicaciones, cambiar fotos mientras están pendientes y reportar la venta de las suyas |
| administrador | solo al arrancar, con `ADMIN_EMAIL` y `ADMIN_PASSWORD` | ver todas las publicaciones y usuarios, aprobar publicaciones pendientes, eliminar publicaciones y usuarios |

- El registro público **nunca** crea administradores, aunque el cliente envíe
  `"rol":"administrador"`: el campo se ignora.
- El administrador no publica vehículos: las rutas de vendedor exigen el rol
  `vendedor` y le responden `403`.

## Cómo se aplica

El control se hace en tres niveles, de lo general a lo particular:

1. **Por rol, en la ruta.** Las rutas protegidas se declaran en grupos con el
   middleware correspondiente; el handler nunca decide si el rol alcanza.

   ```go
   // API (internal/httpapi)
   seller := r.Group("", Authenticate(authSvc), RequireRole(user.RoleVendedor))
   admin  := r.Group("/admin", Authenticate(authSvc), RequireRole(user.RoleAdministrador))

   // Páginas web: mismo control, pero el visitante va a /ingresar
   panel := web.Group("/panel", h.pages.requireRole(user.RoleAdministrador))
   ```

   | Situación | API | Páginas |
   |---|---|---|
   | sin token o token inválido | `401` (+ `WWW-Authenticate: Bearer`) | redirige a `/ingresar` |
   | autenticado con un rol que no corresponde | `403` | página *Tu cuenta no tiene permiso para esto* (403) |

2. **Por dueño, en el servicio.** Que el usuario sea vendedor no basta para
   tocar cualquier publicación: `vehicle.Service` compara `VendedorID` con el
   usuario autenticado en `ReportSale`, `Own`, `AddImages` y `RemoveImage`, y
   devuelve `vehicle.ErrNotOwner` (`403`) si es de otro.

3. **Por estado del recurso.** Un vehículo pendiente o vendido no es público:
   el catálogo y sus fotos responden `404` a quien no sea el dueño o el
   administrador (`vehicle.Service.ImagePath`, `canSeeUnpublished`).

La traducción de estos errores a códigos HTTP está en un solo lugar:
`errorStatus` en `httpapi/errors.go`.

## API de administración

Todas las rutas exigen un usuario autenticado con rol `administrador`.

| Método y ruta | Respuesta |
|---|---|
| `GET /admin/vehiculos` | `200` todas las publicaciones (pendientes, publicadas y vendidas) con el nombre del vendedor, de la más reciente a la más antigua |
| `POST /admin/vehiculos/:id/aprobar` | `200` la publicación en estado `publicada`; `404` si no existe; `409` si no estaba pendiente |
| `DELETE /admin/vehiculos/:id` | `204` elimina la publicación en cualquier estado, con sus fotos; `404` si no existe |
| `GET /admin/usuarios` | `200` los usuarios (id, nombre, email, teléfono, rol, fecha de registro), **sin** `password_hash` |
| `DELETE /admin/usuarios/:id` | `204` elimina al usuario y todas sus publicaciones; `404` si no existe; `409` si es su propia cuenta |

```bash
ADMIN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@automarket.com","password":"clave-admin-123"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

curl -H "Authorization: Bearer $ADMIN" localhost:8080/admin/vehiculos
curl -X POST -H "Authorization: Bearer $ADMIN" localhost:8080/admin/vehiculos/<id>/aprobar
curl -X DELETE -H "Authorization: Bearer $ADMIN" localhost:8080/admin/usuarios/<id>
```

Errores: `401` sin token válido, `403` rol distinto de administrador, `404`,
`409`. Siempre con la forma `{"error":"mensaje"}`.

## Panel web

| Página | Acción |
|---|---|
| `GET /panel` | lista de publicaciones (con sus fotos) y de usuarios |
| `POST /panel/vehiculos/:id/aprobar` | aprueba una pendiente |
| `POST /panel/vehiculos/:id/eliminar` | elimina una publicación |
| `POST /panel/usuarios/:id/eliminar` | elimina un usuario y sus publicaciones |

El panel usa `/panel` porque `/admin` es de la API. Llama al mismo
`admin.Service` que la API, así que las reglas son idénticas. Los formularios
son `POST` y la cookie de sesión es `SameSite=Strict`, por lo que otro sitio no
puede aprobar ni eliminar a nombre del administrador (CSRF).

## Relación con los demás componentes

- **Authentication (integrante 1):** el rol sale del repositorio de usuarios,
  no del cliente ni del token. `Authenticate` vuelve a cargar al usuario en
  cada solicitud, así que al eliminarlo su token deja de servir de inmediato
  (`401`), sin esperar a que expire.
- **Vehículos (integrante 2):** `admin.Service.Approve` usa
  `vehicle.Service.Approve`, que comprueba que esté pendiente y cambia el
  estado bajo el mismo bloqueo (`Repository.Update`). `DeletePublication` y
  `DeleteUser` usan `Delete` y `DeleteBySeller`, que también borran las fotos.
- **Accounting (integrante 4):** el middleware global registra todas las
  solicitudes, también las rechazadas. Cada `401` y `403` queda con la
  operación, el actor, su rol y la IP, de modo que los intentos de acceso no
  autorizados son trazables. Ver [integracion-accounting.md](integracion-accounting.md).

## Pruebas

```bash
go test ./internal/admin/ ./internal/httpapi/ -v -run 'Admin|Owner|Role|Image|EndToEnd'
```

| Prueba | Qué demuestra |
|---|---|
| `TestAdminRoutesRequireAdministrator` | cada ruta `/admin` responde `401` al visitante y `403` al vendedor, y los intentos rechazados no cambian nada |
| `TestAdminApprovesPendingVehicle` | criterio de HU-ADM-01: la pendiente pasa a publicada y aparece en el catálogo |
| `TestAdminDeletesPublication` | la publicación desaparece del catálogo |
| `TestAdminDeletesUserAndRevokesAccess` | se borran el usuario y sus publicaciones, y su token deja de servir |
| `TestAdminUserListNeverExposesPasswordHash` | la lista de usuarios no expone el hash de la contraseña |
| `TestReportSaleOnlyByOwner` | un vendedor no puede reportar la venta de un vehículo ajeno (`403`) |
| `TestImageAccess` | las fotos de una pendiente solo las ven su dueño y el administrador |
| `TestWebSellerPagesRequireSellerRole`, `TestWebAdminPanel` | el mismo control en las páginas web |
| `internal/admin/service_test.go` | aprobar, listar en todos los estados y eliminar, a nivel de servicio |
| `TestAAAEndToEnd` | incluye el paso *Ana intenta aprobar su propia publicación* → `403`, registrado en accounting |

## Decisiones de diseño

- **Denegar por defecto:** una ruta protegida se declara dentro de un grupo con
  `Authenticate` + `RequireRole`; el handler da por hecho que el solicitante ya
  fue autorizado y no repite la comprobación.
- **`401` y `403` son distintos:** `401` es "no sé quién eres" (falta o falla
  el token) y `403` es "sé quién eres, pero tu rol no puede hacer esto".
- **`404` en lugar de `403` para lo que no es público:** responder `403` a un
  vehículo pendiente revelaría que existe. Se responde igual que a uno
  inexistente.
- **El administrador no puede eliminarse a sí mismo** (`admin.ErrSelfDelete`,
  `409`), para que el sistema no se quede sin nadie que pueda moderar.
- **Al eliminar un usuario se borran primero sus publicaciones:** si algo falla
  a mitad, no quedan vehículos de un vendedor inexistente en el catálogo.
- **Aprobar comprueba el estado dentro del bloqueo:** si el vendedor reporta la
  venta al mismo tiempo que el administrador aprueba, no puede quedar
  "publicada" una publicación ya vendida.
