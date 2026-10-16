package main

import (
	"crypto/rand"
	"log"
	"os"
	"time"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/accounting"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/admin"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/httpapi"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/imagestore"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/vehicle"
)

const tokenTTL = time.Hour

func main() {
	users, err := user.NewFileRepository(env("DATA_FILE", "data/users.json"))
	if err != nil {
		log.Fatalf("abriendo usuarios: %v", err)
	}
	vehicles, err := vehicle.NewFileRepository(env("VEHICLES_FILE", "data/vehicles.json"))
	if err != nil {
		log.Fatalf("abriendo vehículos: %v", err)
	}

	authSvc := auth.NewService(users, auth.NewTokenManager(authSecret(), tokenTTL))
	ensureAdmin(authSvc)
	images := imagestore.New(env("IMAGES_DIR", "data/imagenes"))
	vehicleSvc := vehicle.NewService(vehicles, users, images)

	accLog := openAccounting()
	defer accLog.Close()

	r := httpapi.NewRouter(httpapi.Services{
		Auth:       authSvc,
		Vehicles:   vehicleSvc,
		Admin:      admin.NewService(users, vehicleSvc),
		Accounting: accLog,
	})

	addr := ":" + env("PORT", "8080")
	log.Printf("AutoMarket escuchando en %s", addr)
	log.Fatal(r.Run(addr))
}

// authSecret lee AUTH_SECRET. Si no está definido genera uno aleatorio, con lo
// cual los tokens dejan de valer cuando el servidor se reinicia.
func authSecret() []byte {
	if s := os.Getenv("AUTH_SECRET"); s != "" {
		return []byte(s)
	}
	log.Println("AUTH_SECRET no definido: usando una clave aleatoria (los tokens no sobreviven a un reinicio)")
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatalf("generando clave: %v", err)
	}
	return secret
}

// openAccounting abre el registro de accounting con ACCOUNTING_SECRET. La
// clave es obligatoria y debe ser estable: con ella se firma cada entrada y se
// verifica la cadena al arrancar. Si el registro fue alterado, el servidor no
// arranca e indica la línea.
func openAccounting() *accounting.Log {
	secret := os.Getenv("ACCOUNTING_SECRET")
	if secret == "" {
		log.Fatal("ACCOUNTING_SECRET no definido: es la clave que protege la integridad del registro de accounting")
	}
	path := env("ACCOUNTING_FILE", "data/accounting.log")
	l, err := accounting.Open(path, []byte(secret))
	if err != nil {
		log.Fatalf("abriendo %s: %v", path, err)
	}
	return l
}

// ensureAdmin crea el administrador con ADMIN_EMAIL y ADMIN_PASSWORD (y
// ADMIN_NOMBRE opcional) si todavía no existe. Sin esas variables el sistema
// funciona, pero nadie puede aprobar publicaciones.
func ensureAdmin(svc *auth.Service) {
	email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		log.Println("ADMIN_EMAIL o ADMIN_PASSWORD no definidos: no se crea administrador")
		return
	}
	if err := svc.EnsureAdmin(env("ADMIN_NOMBRE", "Administrador"), email, password); err != nil {
		log.Fatalf("creando administrador: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
