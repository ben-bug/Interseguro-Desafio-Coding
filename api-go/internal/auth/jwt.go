// Package auth encapsula la emisión y verificación de tokens JWT.
//
// Se aísla del framework HTTP a propósito: el middleware de Fiber y el handler
// de login consumen el mismo Manager, de modo que existe un único lugar donde
// se define qué es un token válido.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrTokenExpired indica un token bien formado y correctamente firmado
	// cuya vigencia ya venció. Se distingue del token inválido porque el
	// cliente puede resolverlo simplemente renovando la sesión.
	ErrTokenExpired = errors.New("el token expiró")
	// ErrTokenInvalid cubre firma incorrecta, algoritmo no permitido, claims
	// que no coinciden y token malformado.
	ErrTokenInvalid = errors.New("el token es inválido")
)

// Manager emite y verifica tokens HS256.
type Manager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewManager construye un Manager con el secreto compartido y los claims de
// identidad del emisor.
func NewManager(secret, issuer, audience string, ttl time.Duration) *Manager {
	return &Manager{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
	}
}

// TTL expone la vigencia configurada, para informarla en la respuesta de login.
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue firma un token para el sujeto dado y devuelve también su instante de
// expiración.
func (m *Manager) Issue(subject string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.ttl)

	claims := jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    m.issuer,
		Audience:  jwt.ClaimStrings{m.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firmando el token: %w", err)
	}
	return signed, expiresAt, nil
}

// Verify valida la firma y los claims del token, devolviendo el sujeto.
//
// El algoritmo se restringe explícitamente a HS256: sin esa restricción, un
// atacante podría presentar un token con `alg: none` o forzar una confusión
// entre algoritmos simétricos y asimétricos para que la clave pública se
// interprete como secreto HMAC.
func (m *Manager) Verify(token string) (string, error) {
	parsed, err := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrTokenExpired
		}
		return "", fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", ErrTokenInvalid
	}
	return claims.Subject, nil
}
