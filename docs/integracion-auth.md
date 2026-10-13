# Autenticación: guía de integración (Integrante 1)

Cubre HU-VEN-01 (registro autónomo) y la parte *Authentication* de RF-01.

## Ejecutar

```bash
go mod tidy                      # primera vez: descarga Gin y genera go.sum
go test ./...
AUTH_SECRET=cambia-esto ADMIN_EMAIL=admin@automarket.com ADMIN_PASSWORD=admin-clave-1 \
  go run ./cmd/server
```

| Variable | Obligatoria | Por defecto | Uso |
|---|---|---|---|
| `AUTH_SECRET` | recomendada | aleatoria al arrancar | Clave HMAC de los tokens. Sin ella, los tokens no sobreviven a un reinicio. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | no | — | Crea el administrador inicial si no existe. Es la única forma de tener un administrador. |
| `DATA_FILE` | no | `data/users.json` | Archivo de usuarios. |
| `PORT` | no | `8080` | Puerto HTTP. |

## Endpoints

Ambos son públicos.

```bash
# Registro: siempre crea un usuario con rol "vendedor"
curl -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana Pérez","email":"ana@correo.com","telefono":"3001234567","password":"clave-segura-1"}'
# 201 {"id":"…","nombre":"Ana Pérez","email":"ana@correo.com","telefono":"3001234567","rol":"vendedor"}

# Login
curl -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"ana@correo.com","password":"clave-segura-1"}'
# 200 {"token":"…","tipo":"Bearer","expira_en":"…"}

# Ruta protegida
curl localhost:8080/ruta -H 'Authorization: Bearer <token>'
```

Errores: `400` datos inválidos, `409` correo repetido, `401` credenciales o token inválidos (con `WWW-Authenticate: Bearer`), `500` error interno.
Todos tienen la forma `{"error":"mensaje"}`.

## Cómo identificar al solicitante en rutas protegidas

```go
protected := r.Group("/", httpapi.Authenticate(authSvc)) // 401 si no hay token válido

protected.POST("/vehiculos", func(c *gin.Context) {
    u, _ := httpapi.CurrentUser(c) // u.ID, u.Rol, u.Telefono…
})
```

- **El rol nunca viene del cliente ni del token.** El token solo lleva `sub` (ID de usuario) y `exp`; `Authenticate` carga el usuario del repositorio en cada solicitud y de ahí sale el rol. Si un administrador elimina a un usuario, su token deja de servir de inmediato.
- **Visitante** = ruta sin `Authenticate`.

## Qué necesita cada integrante

- **Integrante 2 (vehículos):** use `u.ID` como `vendedorID` de la publicación. Para el contacto del vendedor en el catálogo público use `user.Repository.FindByID` y exponga `Nombre`, `Email` y `Telefono` (nunca `PasswordHash`).
- **Integrante 3 (autorización):** escriba `RequireRole(roles ...user.Role)` sobre `CurrentUser(c)` (403 si el rol no está permitido). Para eliminar usuarios, agregue `Delete(id string) error` a `user.Repository` y a `FileRepository` (mismo patrón que `Create`: bloquear, modificar, `persist`).
- **Integrante 4 (accounting):** registre `register` y `login` como operaciones con resultado (éxito, 400, 401, 409). El actor de `register` y `login` fallido es anónimo; en rutas protegidas sale de `CurrentUser(c)`. Un middleware global que envuelva `c.Next()` ve el usuario si `Authenticate` ya corrió.

## Decisiones de diseño

- Contraseñas: PBKDF2-HMAC-SHA256 (`crypto/pbkdf2`), 210 000 iteraciones, sal aleatoria de 16 bytes, comparación en tiempo constante.
- Tokens: `base64url(claims).base64url(HMAC-SHA256)`, vigencia 1 hora.
- El login responde igual si el correo no existe o la clave es incorrecta, y gasta el mismo tiempo de cómputo en ambos casos.
- Persistencia: JSON con mutex y escritura atómica (archivo temporal + `rename`).
- El administrador inicial por variables de entorno no está en las HU; se agregó porque sin él no puede existir un administrador autenticado. Si el equipo prefiere otra forma, se quita `seedAdmin` y `Service.SeedAdmin`.
