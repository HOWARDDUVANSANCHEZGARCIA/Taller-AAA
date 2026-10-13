# Taller-AAA · AutoMarket

API backend en Go (solo standard library + Gin) con el marco de seguridad AAA
(Authentication, Authorization, Accounting) para un sitio de venta de vehículos usados.

```
cmd/server/        punto de entrada
internal/user/     modelo de usuario y persistencia
internal/auth/     registro, login, hash de contraseñas y tokens
internal/httpapi/  rutas, handlers y middleware (Gin)
docs/              guías de integración
```

- Autenticación: [docs/integracion-auth.md](docs/integracion-auth.md)
