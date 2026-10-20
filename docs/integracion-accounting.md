# Accounting: guía de integración

Cubre la parte *Accounting* de RF-01: cada operación queda registrada de forma
**trazable** (quién, qué, sobre qué recurso, con qué resultado y cuándo) e
**íntegra** (cualquier cambio posterior al registro se detecta).

## Ejecutar

```bash
go mod tidy                      # primera vez
go test ./...
AUTH_SECRET=clave-tokens ACCOUNTING_SECRET=clave-accounting go run ./cmd/server
```

| Variable | Obligatoria | Por defecto | Uso |
|---|---|---|---|
| `ACCOUNTING_SECRET` | sí | — | Clave HMAC con la que se firma cada entrada. Sin ella el servidor no arranca. Debe ser siempre la misma: con otra clave el registro existente se considera alterado. |
| `ACCOUNTING_FILE` | no | `data/accounting.log` | Archivo del registro. |

Las demás variables están en [guia-ejecucion.md](guia-ejecucion.md).

## Formato de cada entrada

El registro es un archivo de texto con una entrada JSON por línea (JSON Lines).
Solo se agregan líneas al final.

| Campo | Contenido |
|---|---|
| `seq` | número consecutivo de la entrada, desde 1 |
| `momento` | fecha y hora en UTC, con nanosegundos |
| `operacion` | método y ruta del servidor, p. ej. `POST /admin/vehiculos/:id/aprobar` |
| `recurso` | ruta concreta solicitada, p. ej. `/admin/vehiculos/3f2a…/aprobar` |
| `actor` | ID del usuario autenticado, o `anonimo` |
| `rol` | `visitante`, `vendedor` o `administrador` |
| `resultado` | código HTTP de la respuesta (`200`, `401`, `403`, `404`…) |
| `ip` | IP de la conexión |
| `hash_previo` | `hash` de la entrada anterior (vacío en la primera) |
| `hash` | HMAC-SHA256 de todos los campos anteriores |

```json
{"seq":7,"momento":"2026-10-16T03:30:12.4410Z","operacion":"POST /admin/vehiculos/:id/aprobar","recurso":"/admin/vehiculos/417bf233…/aprobar","actor":"c4e81a9f…","rol":"administrador","resultado":200,"ip":"::1","hash_previo":"ddb29fa0…","hash":"8c1e47d2…"}
```

## Qué se registra

- **Todas las solicitudes** de la API y de las páginas web, también las
  rechazadas (`401`, `403`, `404`) y las rutas inexistentes
  (`operacion: "GET (ruta inexistente)"`). Los intentos de acceso no
  autorizados quedan trazados con el actor y su rol.
- **Ver una foto** también es una operación sobre un recurso: queda
  `GET /vehiculos/:id/imagenes/:nombre` con su resultado.
- **No se registra** la hoja de estilos (`/static/…`): no es una operación.
- **Actor:**
  - en rutas protegidas, el usuario que dejó `Authenticate` (`CurrentUser(c)`);
  - en el login exitoso, el usuario que ingresó;
  - en el registro, el login fallido y las rutas públicas sin sesión, `anonimo`.
- **Nunca** se guardan contraseñas, tokens ni el contenido del formulario: solo
  los campos de la tabla anterior.
- **IP confiable:** `NewRouter` llama a `SetTrustedProxies(nil)`, así que
  `c.ClientIP()` usa la IP real de la conexión e ignora `X-Forwarded-For`, que
  el cliente puede falsificar.

## Integridad

Cada entrada lleva:

```
hash = HMAC-SHA256(ACCOUNTING_SECRET, [seq, momento, operacion, recurso, actor, rol, resultado, ip, hash_previo])
```

Como cada `hash` incluye el `hash_previo`, las entradas forman una cadena: si
alguien cambia una línea, la firma de esa línea deja de coincidir; si borra,
inserta o reordena líneas, se rompe la secuencia de `seq` o de `hash_previo`.
Sin la clave no se puede recalcular la cadena para disimular el cambio.

**Al arrancar**, `accounting.Open` verifica todo el archivo. Si está alterado,
el servidor **no arranca** e indica la línea:

```
registro de accounting alterado en la línea N: el contenido no coincide con su firma
```

Otros mensajes posibles: `no continúa la cadena de la línea anterior`,
`se esperaba la entrada N y aparece la M (faltan o sobran líneas)`,
`no es una entrada válida`.

Para la demostración, ver el paso 9 de [guia-ejecucion.md](guia-ejecucion.md).

## Uso desde el código

```go
// cmd/server/main.go
accLog, err := accounting.Open("data/accounting.log", []byte(secret)) // verifica la cadena
defer accLog.Close()

// internal/httpapi/router.go: va primero para envolver todas las rutas
r.Use(Accounting(accLog))

// Verificar un archivo sin abrirlo para escribir
n, err := accounting.Verify("data/accounting.log", []byte(secret)) // n = número de entradas
```

- **Una ruta nueva no necesita nada:** el middleware global la registra sola.
  Para que quede el actor, la ruta debe pasar por `Authenticate` u
  `OptionalAuthenticate` (o, como el login, guardar al usuario con
  `c.Set(currentUserKey, u)`).
- El middleware registra **después** de procesar la solicitud
  (`c.Next()` primero), cuando ya se conocen el usuario y el resultado.
- Si escribir la entrada falla, la respuesta ya se envió; el error queda en el
  log del servidor para que el operador lo revise.

## Estructura común e integración

- `NewRouter` (`internal/httpapi/router.go`) arma todas las rutas y lo usan
  tanto `main` como las pruebas, para que ambos ejecuten la misma
  configuración.
- `errorStatus` (`internal/httpapi/errors.go`) traduce los errores de todos los
  componentes a códigos HTTP en un solo lugar, para la API y para las páginas.
- `jsonfile` (carga y escritura atómica) e `ids` (identificadores aleatorios)
  son compartidos por los repositorios de usuarios y de vehículos.

## Pruebas

```bash
go test ./internal/accounting/ -v
go test ./internal/httpapi/ -v -run 'EndToEnd|Accounting'
```

| Prueba | Qué demuestra |
|---|---|
| `TestDetectsTampering` | se detecta cambiar el resultado o el actor, borrar la primera línea o una del medio, intercambiar dos líneas, duplicar una y escribir una línea basura; `Open` se niega a abrir el archivo |
| `TestWrongKeyIsDetected` | con otra clave el registro no se acepta |
| `TestEmptyKeyIsRejected` | no se puede abrir el registro sin clave |
| `TestChainContinuesAcrossRestarts` | al reiniciar el servidor la cadena continúa donde iba |
| `TestConcurrentRecordsKeepChain` | muchas solicitudes simultáneas no rompen la cadena |
| `TestAAAEndToEnd` | recorre las tres historias de usuario por la API y comprueba, para cada paso, la entrada con su actor, rol, resultado, recurso, momento e IP; que la cadena esté íntegra, y que el registro no contenga contraseñas ni tokens |
| `TestAccountingRecordsWebPagesButNotStylesheet` | las páginas web y las rutas inexistentes se registran; la hoja de estilos no |
| `TestAccountingIgnoresForwardedFor` | un cliente no puede falsificar su IP con `X-Forwarded-For` |
| `TestAccountingRecordsImageViews` | ver una foto queda registrado con el actor y el resultado |

## Decisiones de diseño

- **HMAC y no solo SHA-256:** con un hash sin clave, quien edite el archivo
  podría recalcular toda la cadena. Con HMAC necesita además
  `ACCOUNTING_SECRET`, que no está en el repositorio ni en el archivo.
- **Firma sin ambigüedad:** los campos se firman como un arreglo JSON, así no
  se pueden "correr" caracteres de un campo a otro y conservar la firma.
- **Solo agregar:** el archivo se abre con `O_APPEND` y permisos `0600`; cada
  entrada se escribe con `Sync` para que no se pierda si el proceso se cae.
- **Un mutex** protege el número de secuencia y el último hash, para que las
  solicitudes simultáneas no generen dos entradas con el mismo `seq`.
- **`operacion` usa la ruta del servidor** (`/vehiculos/:id/venta`) para poder
  agrupar por tipo de operación, y **`recurso` la ruta concreta** para saber
  sobre qué publicación o usuario se actuó.
- **El actor es el ID**, no el nombre ni el correo: el registro no repite datos
  personales que ya están en `users.json`.
- **Limitación conocida:** borrar las últimas líneas del archivo no rompe la
  cadena de las que quedan, así que no se detecta solo con la verificación. Se
  podría cubrir guardando aparte (fuera del servidor) el último `seq` y `hash`
  o enviando copias del registro a otro sistema. Igualmente, quien tenga
  `ACCOUNTING_SECRET` puede reescribir la cadena: la clave debe protegerse como
  una contraseña.
