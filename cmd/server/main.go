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
	authSvc := auth.NewService(users, tokens)

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

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
