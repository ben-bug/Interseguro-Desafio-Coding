// Package api contiene la capa HTTP de la API Go: rutas, handlers, middleware
// y los contratos de entrada y salida.
package api

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Códigos de error de la capa HTTP. Son estables y forman parte del contrato
// público: el cliente puede ramificar sobre ellos sin parsear el mensaje, que
// está escrito para personas y puede cambiar sin previo aviso.
//
// Los códigos de validación de la matriz (EMPTY_MATRIX, RAGGED_ROWS,
// NON_FINITE_VALUE, MATRIX_TOO_LARGE) los define el paquete matrix y se
// propagan tal cual.
const (
	CodeInvalidBody         = "INVALID_BODY"
	CodeInvalidCredentials  = "INVALID_CREDENTIALS"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeTokenExpired        = "TOKEN_EXPIRED"
	CodeUpstreamUnavailable = "UPSTREAM_UNAVAILABLE"
	CodeUpstreamTimeout     = "UPSTREAM_TIMEOUT"
	CodeUpstreamError       = "UPSTREAM_ERROR"
	CodeNotFound            = "NOT_FOUND"
	CodeInternal            = "INTERNAL_ERROR"
)

// ErrorPayload es el cuerpo de un error. Ambas APIs del sistema usan esta misma
// forma, de modo que el frontend tiene un único camino de manejo de errores.
type ErrorPayload struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId,omitempty"`
}

// ErrorResponse envuelve el payload bajo la clave `error`, para que nunca se
// confunda con una respuesta exitosa.
type ErrorResponse struct {
	Error ErrorPayload `json:"error"`
}

// APIError es un error que ya sabe con qué status HTTP debe responderse.
type APIError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *APIError) Error() string { return e.Message }

// NewAPIError construye un error de la capa HTTP.
func NewAPIError(status int, code, message string, details map[string]any) *APIError {
	return &APIError{Status: status, Code: code, Message: message, Details: details}
}

// ErrorHandler es el punto único de conversión de errores a respuestas HTTP.
//
// Centralizarlo evita que cada handler repita la serialización y garantiza que
// ningún error se escape con un formato distinto: cualquier error que llegue
// aquí sale como ErrorResponse, incluidos los panics recuperados y las rutas
// inexistentes.
func ErrorHandler(c fiber.Ctx, err error) error {
	status, payload := resolveError(err)
	payload.RequestID = requestid.FromContext(c)

	return c.Status(status).JSON(ErrorResponse{Error: payload})
}

// resolveError traduce un error al status HTTP y al cuerpo con que debe
// responderse.
//
// Es la única fuente de esa decisión. ErrorHandler la usa para escribir la
// respuesta y el middleware de registro para saber con qué status terminará el
// request, de modo que log y respuesta no pueden discrepar: si la traducción
// cambia, cambia para ambos a la vez.
func resolveError(err error) (int, ErrorPayload) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status, ErrorPayload{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Details: apiErr.Details,
		}
	}

	// Errores del propio framework: sobre todo el 404 de ruta inexistente y el
	// 405 de método no permitido.
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code := CodeInternal
		if fiberErr.Code == http.StatusNotFound {
			code = CodeNotFound
		}
		return fiberErr.Code, ErrorPayload{Code: code, Message: fiberErr.Message}
	}

	// Lo no contemplado sale como 500 genérico a propósito: el detalle interno
	// queda en los logs del servidor y no se filtra al cliente.
	return http.StatusInternalServerError, ErrorPayload{
		Code:    CodeInternal,
		Message: "error interno del servidor",
	}
}

// statusFromError expone solo el status con que se responderá. Lo usa el
// middleware de registro, que necesita el código pero no el cuerpo.
func statusFromError(err error) int {
	status, _ := resolveError(err)
	return status
}
