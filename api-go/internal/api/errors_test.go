package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// TestResolveError cubre la traducción de error a respuesta, que es el punto
// del que dependen tanto el cuerpo que recibe el cliente como el status que
// registra el log.
func TestResolveError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
		wantDetails bool
	}{
		{
			name: "error de la API conserva código, mensaje y detalles",
			err: NewAPIError(http.StatusBadRequest, "RAGGED_ROWS", "filas desiguales",
				map[string]any{"rowIndex": 2}),
			wantStatus:  http.StatusBadRequest,
			wantCode:    "RAGGED_ROWS",
			wantMessage: "filas desiguales",
			wantDetails: true,
		},
		{
			name:       "error de la API sin detalles",
			err:        NewAPIError(http.StatusBadGateway, CodeUpstreamError, "upstream", nil),
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeUpstreamError,
		},
		{
			name:       "404 del framework se traduce a NOT_FOUND",
			err:        fiber.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   CodeNotFound,
		},
		{
			// Un 405 no tiene código propio: sale como error interno de cara al
			// catálogo, pero conserva su status real.
			name:       "otro error del framework conserva su status",
			err:        fiber.ErrMethodNotAllowed,
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   CodeInternal,
		},
		{
			name:        "error desconocido no filtra su mensaje",
			err:         errors.New("connection to database at 10.0.0.5 refused"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    CodeInternal,
			wantMessage: "error interno del servidor",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, payload := resolveError(tc.err)

			if status != tc.wantStatus {
				t.Errorf("status = %d, se esperaba %d", status, tc.wantStatus)
			}
			if payload.Code != tc.wantCode {
				t.Errorf("código = %q, se esperaba %q", payload.Code, tc.wantCode)
			}
			if tc.wantMessage != "" && payload.Message != tc.wantMessage {
				t.Errorf("mensaje = %q, se esperaba %q", payload.Message, tc.wantMessage)
			}
			if payload.Message == "" {
				t.Error("el payload no trae mensaje legible")
			}
			if got := payload.Details != nil; got != tc.wantDetails {
				t.Errorf("presencia de detalles = %v, se esperaba %v", got, tc.wantDetails)
			}
		})
	}
}

// TestResolveErrorHidesInternalDetail comprueba explícitamente que el mensaje de
// un error inesperado no llegue al cliente: podría contener rutas de archivos,
// nombres de host o estructura interna útil para un atacante.
func TestResolveErrorHidesInternalDetail(t *testing.T) {
	secret := "panic: runtime error at /home/app/internal/db/conn.go:42"

	_, payload := resolveError(errors.New(secret))

	if payload.Message == secret {
		t.Fatal("el mensaje interno se está devolviendo al cliente")
	}
}

// TestStatusFromErrorMatchesResolveError verifica que el status que registra el
// log sea el mismo que se escribe en la respuesta. Si ambos se calcularan por
// separado podrían divergir, y el log dejaría de servir para diagnosticar.
func TestStatusFromErrorMatchesResolveError(t *testing.T) {
	cases := []error{
		NewAPIError(http.StatusUnauthorized, CodeUnauthorized, "sin token", nil),
		fiber.ErrNotFound,
		fiber.ErrTeapot,
		errors.New("inesperado"),
		nil,
	}

	for _, err := range cases {
		want, _ := resolveError(err)
		if got := statusFromError(err); got != want {
			t.Errorf("statusFromError(%v) = %d, resolveError devuelve %d", err, got, want)
		}
	}
}
