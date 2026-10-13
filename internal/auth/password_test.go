package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("una-clave-segura")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("una-clave-segura", hash) {
		t.Error("la contraseña correcta debe verificar")
	}
	if VerifyPassword("otra-clave", hash) {
		t.Error("una contraseña incorrecta no debe verificar")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	a, _ := HashPassword("misma-clave")
	b, _ := HashPassword("misma-clave")
	if a == b {
		t.Error("dos hashes de la misma contraseña deben diferir por la sal")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "texto-plano", "pbkdf2-sha256$x$y$z", "md5$1$a$b", "pbkdf2-sha256$0$AA$AA"} {
		if VerifyPassword("clave", bad) {
			t.Errorf("un hash mal formado no debe verificar: %q", bad)
		}
	}
}
