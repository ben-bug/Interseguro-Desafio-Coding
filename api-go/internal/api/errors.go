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
	payload := ErrorPayload{
		Code:      CodeInternal,
		Message:   "error interno del servidor",
		RequestID: requestid.FromContext(c),
	}
	status := statusFromError(err)

	var apiErr *APIError
	var fiberErr *fiber.Error

	switch {
	case errors.As(err, &apiErr):
		status = apiErr.Status
		payload.Code = apiErr.Code
		payload.Message = apiErr.Message
		payload.Details = apiErr.Details

	case errors.As(err, &fiberErr):
		// Errores generados por el propio framework: sobre todo el 404 de ruta
		// no encontrada y el 405 de método no permitido.
		status = fiberErr.Code
		payload.Message = fiberErr.Message
		if status == http.StatusNotFound {
			payload.Code = CodeNotFound
		}
	}

	// Los errores no contemplados salen como 500 genérico a propósito: el
	// detalle interno queda en los logs del servidor y no se filtra al cliente.
	return c.Status(status).JSON(ErrorResponse{Error: payload})
}

// statusFromError permite que el logger conozca el estado que ErrorHandler
// escribirá después de que la cadena de middleware termine.
func statusFromError(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code
	}

	return http.StatusInternalServerError
}
