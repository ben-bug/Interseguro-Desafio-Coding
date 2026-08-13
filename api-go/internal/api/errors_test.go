package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestStatusFromError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"APIError", NewAPIError(http.StatusBadGateway, CodeUpstreamError, "upstream", nil), http.StatusBadGateway},
		{"FiberError", fiber.ErrNotFound, http.StatusNotFound},
		{"desconocido", errors.New("fallo"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusFromError(tc.err); got != tc.want {
				t.Fatalf("statusFromError() = %d, se esperaba %d", got, tc.want)
			}
		})
	}
}
