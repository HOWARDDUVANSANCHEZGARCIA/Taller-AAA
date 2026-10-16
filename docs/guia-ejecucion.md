# Guía de ejecución

## Requisitos

Go 1.24 o superior. La única dependencia externa es Gin (v1.11.0).

## Primera vez

```bash
go mod tidy        # descarga Gin y genera go.sum
go test ./...      # todas las pruebas, incluida la de integración AAA
```

## Ejecutar

```bash
AUTH_SECRET=clave-tokens \
ACCOUNTING_SECRET=clave-accounting \
ADMIN_EMAIL=admin@automarket.com \
ADMIN_PASSWORD=clave-admin-123 \
go run ./cmd/server
```

En Windows (PowerShell):

```powershell
$env:AUTH_SECRET="clave-tokens"; $env:ACCOUNTING_SECRET="clave-accounting"
$env:ADMIN_EMAIL="admin@automarket.com"; $env:ADMIN_PASSWORD="clave-admin-123"
go run ./cmd/server
```

Sitio: http://localhost:8080 · API: mismas rutas, en JSON.

| Variable | Obligatoria | Por defecto | Uso |
|---|---|---|---|
| `ACCOUNTING_SECRET` | sí | — | Clave HMAC del registro de accounting. Debe ser siempre la misma. |
| `AUTH_SECRET` | recomendada | aleatoria | Clave HMAC de los tokens. Sin ella, las sesiones se pierden al reiniciar. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | para moderar | — | Crean el administrador al arrancar si no existe. |
| `ADMIN_NOMBRE` | no | `Administrador` | Nombre del administrador. |
| `PORT` | no | `8080` | Puerto HTTP. |
| `DATA_FILE` | no | `data/users.json` | Usuarios. |
| `VEHICLES_FILE` | no | `data/vehicles.json` | Vehículos. |
| `IMAGES_DIR` | no | `data/imagenes` | Fotos de los vehículos. |
| `ACCOUNTING_FILE` | no | `data/accounting.log` | Registro de accounting. |

Para empezar de cero, borre la carpeta `data/`.

## Recorrido para la demostración

1. **Visitante:** abrir `/`. El catálogo está vacío.
2. **Registro y autenticación:** "Vender mi carro", crear la cuenta e ingresar.
3. **Publicar:** "Publicar vehículo" con sus datos y de 1 a 8 fotos (JPG o PNG). Queda *Pendiente de aprobación* en "Mis vehículos" y no aparece en el catálogo. Mostrar también que un archivo que no es imagen se rechaza.
   - **Fotos privadas:** copiar la dirección de una foto, abrirla en una ventana de incógnito (sin sesión) y ver que responde que no existe.
   - **Cambiar fotos:** en "Mis vehículos", "Cambiar fotos" para agregar o quitar mientras está pendiente.
4. **Autorización:** con la sesión del vendedor, abrir `/panel`: responde *Tu cuenta no tiene permiso para esto* (403).
5. **Administrador:** salir, ingresar con `ADMIN_EMAIL`, abrir "Panel de administración", revisar las fotos y aprobar. Ahora aparece en el catálogo con sus fotos y el contacto del vendedor, y el vendedor ya no puede cambiar las fotos.
   - **Galería:** en el detalle, pasar fotos con las flechas o las miniaturas, hacer clic para verla a pantalla completa.
6. **Venta:** volver como vendedor y "Reportar venta". Sale del catálogo.
7. **Eliminar usuario:** como administrador, eliminar al vendedor. Su sesión deja de servir.
8. **Accounting:** abrir `data/accounting.log`. Cada línea muestra operación, recurso, actor, rol, resultado, IP y momento.
9. **Integridad:** detener el servidor, cambiar un `"resultado":403` por `200` en el log y volver a arrancar. El servidor se niega a iniciar:

   ```
   registro de accounting alterado en la línea N: el contenido no coincide con su firma
   ```

   Restaurar la línea (o borrar `data/`) para continuar.

## Probar la API con curl

```bash
# Registro y login
curl -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana Pérez","email":"ana@correo.com","telefono":"3001234567","password":"clave-segura-1"}'
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"ana@correo.com","password":"clave-segura-1"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

# Publicar (vendedor)
curl -X POST localhost:8080/vehiculos -H "Authorization: Bearer $TOKEN" \
  -F marca=Renault -F modelo=Koleos -F anio=2019 -F precio=78000000 -F kilometraje=60000 \
  -F imagenes=@frente.jpg -F imagenes=@interior.png

# Aprobar (administrador)
ADMIN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@automarket.com","password":"clave-admin-123"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
curl -H "Authorization: Bearer $ADMIN" localhost:8080/admin/vehiculos
curl -X POST -H "Authorization: Bearer $ADMIN" localhost:8080/admin/vehiculos/<id>/aprobar

# Catálogo (visitante)
curl localhost:8080/vehiculos
```

## Rutas

| Ruta | Rol |
|---|---|
| `POST /auth/register`, `POST /auth/login` | visitante |
| `GET /vehiculos`, `GET /vehiculos/:id` | visitante |
| `GET /vehiculos/:id/imagenes/:nombre` | visitante si está publicada; si no, dueño o administrador |
| `POST /vehiculos`, `POST /vehiculos/:id/venta`, `GET /vendedor/vehiculos` | vendedor |
| `POST /vehiculos/:id/imagenes`, `DELETE /vehiculos/:id/imagenes/:nombre` | vendedor dueño, solo pendiente |
| `GET /admin/vehiculos`, `POST /admin/vehiculos/:id/aprobar`, `DELETE /admin/vehiculos/:id` | administrador |
| `GET /admin/usuarios`, `DELETE /admin/usuarios/:id` | administrador |
| Páginas: `/`, `/catalogo/:id`, `/ingresar`, `/registro` | visitante |
| Páginas: `/publicar`, `/mis-vehiculos`, `/mis-vehiculos/:id/fotos` | vendedor |
| Página: `/panel` | administrador |

Errores de la API: `{"error":"mensaje"}` con `400`, `401`, `403`, `404`, `409`, `413` o `500`.
