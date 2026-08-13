package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/socius/interseguro-challenge/api-go/internal/auth"
)

// localsSubject es la clave con que se guarda el sujeto autenticado en el
// contexto del request.
const localsSubject = "authSubject"

// RequireJWT valida el token Bearer del encabezado Authorization.
//
// Devuelve un error en lugar de escribir la respuesta directamente, para que
// ErrorHandler produzca el mismo formato que el resto de la API.
func RequireJWT(manager *auth.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		token, err := bearerToken(c.Get(fiber.HeaderAuthorization))
		if err != nil {
			return NewAPIError(http.StatusUnauthorized, CodeUnauthorized, err.Error(), nil)
		}

		subject, err := manager.Verify(token)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				return NewAPIError(http.StatusUnauthorized, CodeTokenExpired,
					"el token expiró: solicitar uno nuevo en POST /api/v1/auth/login", nil)
			}
			// El motivo exacto del rechazo no se expone: describir por qué una
			// firma no valida le daría a un atacante información útil para
			// afinar el siguiente intento.
			return NewAPIError(http.StatusUnauthorized, CodeUnauthorized, "el token es inválido", nil)
		}

		c.Locals(localsSubject, subject)
		return c.Next()
	}
}

// bearerToken extrae el token del encabezado `Authorization: Bearer <token>`.
// El esquema se compara sin distinguir mayúsculas, como exige RFC 7235.
func bearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("falta el encabezado Authorization")
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", errors.New("el encabezado Authorization debe tener el formato 'Bearer <token>'")
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("el token está vacío")
	}
	return token, nil
}

// RequestLogger emite una línea estructurada por request.
//
// Se prefiere sobre el logger que trae Fiber porque produce JSON con las mismas
// claves que usa la API Node (`requestId`, `method`, `path`, `status`), de modo
// que una sola consulta sirve para correlacionar una traza en ambos servicios.
func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		if err != nil {
			// Fiber ejecuta ErrorHandler después de desenrollar los middleware;
			// en este punto la respuesta aún conserva 200.
			status = statusFromError(err)
		}
		attrs := []any{
			slog.String("requestId", requestid.FromContext(c)),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Float64("durationMs", float64(time.Since(start).Microseconds())/1000),
		}
		if err != nil {
			attrs = append(attrs, slog.Any("error", err))
			logger.Error("request finalizado con error", attrs...)
			return err
		}

		logger.Info("request finalizado", attrs...)
		return nil
	}
}
