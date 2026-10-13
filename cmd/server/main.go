package main

import (
	"crypto/rand"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/auth"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/httpapi"
	"github.com/HOWARDDUVANSANCHEZGARCIA/taller-aaa/internal/user"
)

const tokenTTL = time.Hour

func main() {
	users, err := user.NewFileRepository(env("DATA_FILE", "data/users.json"))
	if err != nil {
		log.Fatalf("abriendo usuarios: %v", err)
	}

	tokens := auth.NewTokenManager(authSecret(), tokenTTL)
	authSvc, err := auth.NewService(users, tokens)
	if err != nil {
		log.Fatalf("creando servicio de autenticación: %v", err)
	}
	seedAdmin(authSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	httpapi.RegisterAuthRoutes(r, authSvc)
	// Las rutas protegidas de los demás componentes se agrupan así:
	//   protected := r.Group("/", httpapi.Authenticate(authSvc))

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

// seedAdmin crea el administrador inicial si se definen ADMIN_EMAIL y ADMIN_PASSWORD.
func seedAdmin(svc *auth.Service) {
	email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		return
	}
	err := svc.SeedAdmin(auth.RegisterInput{
		Nombre:   env("ADMIN_NOMBRE", "Administrador"),
		Email:    email,
		Telefono: env("ADMIN_TELEFONO", "0000000"),
		Password: password,
	})
	if err != nil {
		log.Fatalf("creando administrador inicial: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
