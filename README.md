# Taller-AAA · AutoMarket

API backend en Go (solo standard library + Gin) con el marco de seguridad AAA
(Authentication, Authorization, Accounting) para un sitio de venta de vehículos usados.

```
cmd/server/            punto de entrada
internal/user/         modelo de usuario y persistencia
internal/auth/         registro, login, hash de contraseñas y tokens
internal/vehicle/      publicaciones, estados, fotos y catálogo
internal/imagestore/   fotos en disco: validación JPG/PNG, nombres aleatorios
internal/admin/        moderación de publicaciones y usuarios
internal/accounting/   registro de operaciones encadenado con HMAC
internal/jsonfile/     carga y escritura atómica de archivos JSON
internal/apperr/       errores comunes (validación)
internal/ids/          identificadores aleatorios
internal/testimg/      imágenes pequeñas para las pruebas
internal/httpapi/      rutas, handlers y middleware (Gin): API JSON y páginas web
internal/httpapi/web/  plantillas HTML y CSS (embebidos en el ejecutable)
docs/                  guías de integración
```

```bash
go mod tidy          # primera vez
go test ./...
AUTH_SECRET=clave-tokens ACCOUNTING_SECRET=clave-accounting \
ADMIN_EMAIL=admin@automarket.com ADMIN_PASSWORD=clave-admin-123 \
go run ./cmd/server  # http://localhost:8080
```

- Guía de ejecución y recorrido de demostración: [docs/guia-ejecucion.md](docs/guia-ejecucion.md)
- Diagramas: [estructura](docs/diagramas/estructura.png) y [comportamiento](docs/diagramas/comportamiento.png)

- Autenticación: [docs/integracion-auth.md](docs/integracion-auth.md)
- Vehículos, catálogo y sitio web: [docs/integracion-vehiculos.md](docs/integracion-vehiculos.md)
- Autorización y administración: [docs/integracion-autorizacion.md](docs/integracion-autorizacion.md)
- Accounting: [docs/integracion-accounting.md](docs/integracion-accounting.md)
- Uso de IA: [docs/declaracion-uso-ia.md](docs/declaracion-uso-ia.md)
