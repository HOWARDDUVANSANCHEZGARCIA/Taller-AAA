# Taller-AAA · AutoMarket

API backend en Go (solo standard library + Gin) con el marco de seguridad AAA
(Authentication, Authorization, Accounting) para un sitio de venta de vehículos usados.

```
cmd/server/            punto de entrada
internal/user/         modelo de usuario y persistencia
internal/auth/         registro, login, hash de contraseñas y tokens
internal/vehicle/      publicaciones, estados y catálogo
internal/jsonfile/     carga y escritura atómica de archivos JSON
internal/apperr/       errores comunes (validación)
internal/ids/          identificadores aleatorios
internal/httpapi/      rutas, handlers y middleware (Gin): API JSON y páginas web
internal/httpapi/web/  plantillas HTML y CSS (embebidos en el ejecutable)
docs/                  guías de integración
```

```bash
go mod tidy          # primera vez
go test ./...
AUTH_SECRET=cambia-esto go run ./cmd/server   # http://localhost:8080
```

- Autenticación: [docs/integracion-auth.md](docs/integracion-auth.md)
- Vehículos, catálogo y sitio web: [docs/integracion-vehiculos.md](docs/integracion-vehiculos.md)
