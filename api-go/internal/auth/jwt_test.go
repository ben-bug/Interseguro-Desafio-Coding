package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSecret   = "secreto-de-prueba-suficientemente-largo"
	testIssuer   = "test-issuer"
	testAudience = "test-audience"
)

func newTestManager(ttl time.Duration) *Manager {
	return NewManager(testSecret, testIssuer, testAudience, ttl)
}

func TestIssueAndVerify(t *testing.T) {
	m := newTestManager(15 * time.Minute)

	token, expiresAt, err := m.Issue("demo")
	if err != nil {
		t.Fatalf("Issue devolvió error: %v", err)
	}
	if token == "" {
		t.Fatal("Issue devolvió un token vacío")
	}
	if time.Until(expiresAt) <= 0 {
		t.Errorf("el token nace expirado: expiresAt = %v", expiresAt)
	}

	subject, err := m.Verify(token)
	if err != nil {
		t.Fatalf("Verify rechazó un token recién emitido: %v", err)
	}
	if subject != "demo" {
		t.Errorf("subject = %q, se esperaba %q", subject, "demo")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	// TTL negativo: el token nace vencido, sin necesidad de esperar en el test.
	m := newTestManager(-time.Minute)

	token, _, err := m.Issue("demo")
	if err != nil {
		t.Fatalf("Issue devolvió error: %v", err)
	}

	_, err = m.Verify(token)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("error = %v, se esperaba ErrTokenExpired", err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	issuer := newTestManager(time.Hour)
	token, _, _ := issuer.Issue("demo")

	verifier := NewManager("otro-secreto-distinto", testIssuer, testAudience, time.Hour)

	if _, err := verifier.Verify(token); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("error = %v, se esperaba ErrTokenInvalid", err)
	}
}

// TestVerifyRejectsAlgNone cubre el ataque clásico contra JWT: presentar un
// token con `alg: none` y sin firma para que el verificador lo acepte. La
// restricción explícita de algoritmos en Verify debe bloquearlo.
func TestVerifyRejectsAlgNone(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:   "atacante",
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("no se pudo construir el token del caso de prueba: %v", err)
	}

	if _, err := newTestManager(time.Hour).Verify(unsigned); err == nil {
		t.Fatal("se aceptó un token firmado con alg=none")
	}
}

func TestVerifyRejectsWrongIssuerAndAudience(t *testing.T) {
	cases := []struct {
		name             string
		issuer, audience string
	}{
		{"emisor distinto", "otro-issuer", testAudience},
		{"audiencia distinta", testIssuer, "otra-audiencia"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			foreign := NewManager(testSecret, tc.issuer, tc.audience, time.Hour)
			token, _, _ := foreign.Issue("demo")

			if _, err := newTestManager(time.Hour).Verify(token); !errors.Is(err, ErrTokenInvalid) {
				t.Errorf("error = %v, se esperaba ErrTokenInvalid", err)
			}
		})
	}
}

func TestVerifyRejectsMalformedToken(t *testing.T) {
	cases := []string{
		"",
		"no-es-un-jwt",
		"a.b.c",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJkZW1vIn0", // sin firma
	}

	m := newTestManager(time.Hour)
	for _, token := range cases {
		t.Run(token, func(t *testing.T) {
			if _, err := m.Verify(token); err == nil {
				t.Errorf("se aceptó el token malformado %q", token)
			}
		})
	}
}

func TestTTL(t *testing.T) {
	want := 42 * time.Minute
	if got := newTestManager(want).TTL(); got != want {
		t.Errorf("TTL = %v, se esperaba %v", got, want)
	}
}
