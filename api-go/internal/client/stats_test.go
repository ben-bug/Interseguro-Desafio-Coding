package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/socius/interseguro-challenge/api-go/internal/matrix"
)

// respuesta válida del upstream, con la forma exacta que devuelve la API Node.
const validResponse = `{
  "overall": {"max": 40, "min": 0, "average": 9, "sum": 72, "count": 8},
  "perMatrix": {
    "q": {"max": 1, "min": 0, "average": 0.5, "sum": 2, "count": 4, "rows": 2, "cols": 2, "isSquare": true, "isDiagonal": true, "tolerance": 1e-9}
  },
  "anyDiagonal": true,
  "toleranceFactor": 1e-9
}`

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func testMatrices() map[string]matrix.Matrix {
	return map[string]matrix.Matrix{"q": {{1, 0}, {0, 1}}}
}

func TestNewStatsClientNormalizesBaseURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"sin cambios", "http://localhost:3000", "http://localhost:3000"},
		{"barra final", "http://localhost:3000/", "http://localhost:3000"},
		{"espacios alrededor", "  http://localhost:3000  ", "http://localhost:3000"},
		{"espacios y barra", "  http://localhost:3000/  ", "http://localhost:3000"},
		{"varias barras finales", "http://localhost:3000///", "http://localhost:3000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Una barra sobrante produciría URLs como `…3000//api/v1/statistics`,
			// que algunos servidores rechazan con 404.
			client := NewStatsClient(tc.input, time.Second, 0, discardLogger())

			if client.baseURL != tc.want {
				t.Errorf("baseURL = %q, se esperaba %q", client.baseURL, tc.want)
			}
		})
	}
}

func TestComputeSuccess(t *testing.T) {
	var gotPath, gotAuth, gotRequestID, gotContentType string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotRequestID = r.Header.Get("X-Request-ID")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()

	client := NewStatsClient(server.URL, 2*time.Second, 0, discardLogger())

	stats, err := client.Compute(context.Background(), testMatrices(), "Bearer abc123", "traza-1")
	if err != nil {
		t.Fatalf("Compute devolvió error: %v", err)
	}

	if gotPath != "/api/v1/statistics" {
		t.Errorf("ruta = %q, se esperaba /api/v1/statistics", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	// La API Node exige el mismo token del usuario final y usa el
	// identificador para correlacionar sus logs con los de este servicio.
	if gotAuth != "Bearer abc123" {
		t.Errorf("Authorization = %q, no se propagó el token", gotAuth)
	}
	if gotRequestID != "traza-1" {
		t.Errorf("X-Request-ID = %q, no se propagó", gotRequestID)
	}

	var sent struct {
		Matrices map[string][][]float64 `json:"matrices"`
	}
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("el cuerpo enviado no es JSON válido: %v", err)
	}
	if _, ok := sent.Matrices["q"]; !ok {
		t.Error("la matriz 'q' no llegó en el cuerpo")
	}

	if stats.Overall.Sum != 72 || stats.Overall.Count != 8 {
		t.Errorf("agregados mal deserializados: %+v", stats.Overall)
	}
	if !stats.AnyDiagonal || stats.ToleranceFactor != 1e-9 {
		t.Errorf("anyDiagonal/toleranceFactor mal deserializados: %+v", stats)
	}
	if q := stats.PerMatrix["q"]; !q.IsDiagonal || q.Tolerance != 1e-9 || q.Rows != 2 {
		t.Errorf("perMatrix mal deserializado: %+v", q)
	}
}

// TestComputeOmitsEmptyHeaders comprueba que no se envíen encabezados vacíos:
// un `Authorization:` sin valor es peor que su ausencia, porque algunos
// intermediarios lo tratan como un intento de autenticación fallido.
func TestComputeOmitsEmptyHeaders(t *testing.T) {
	var hasAuth, hasRequestID bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth = r.Header["Authorization"]
		_, hasRequestID = r.Header["X-Request-Id"]
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()

	client := NewStatsClient(server.URL, time.Second, 0, discardLogger())
	if _, err := client.Compute(context.Background(), testMatrices(), "", ""); err != nil {
		t.Fatalf("Compute devolvió error: %v", err)
	}

	if hasAuth {
		t.Error("se envió un encabezado Authorization vacío")
	}
	if hasRequestID {
		t.Error("se envió un encabezado X-Request-ID vacío")
	}
}

func TestComputeErrors(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		assert  func(t *testing.T, err error)
	}{
		{
			name: "respuesta ilegible",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "<html>error del proxy</html>")
			},
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUpstreamUnavailable) {
					t.Errorf("error = %v, se esperaba ErrUpstreamUnavailable", err)
				}
			},
		},
		{
			name: "status 500",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, "boom")
			},
			assert: func(t *testing.T, err error) {
				var statusErr *UpstreamStatusError
				if !errors.As(err, &statusErr) {
					t.Fatalf("error = %v, se esperaba UpstreamStatusError", err)
				}
				if statusErr.Status != http.StatusInternalServerError {
					t.Errorf("status = %d", statusErr.Status)
				}
				// El cuerpo se conserva para poder diagnosticar sin tener que
				// entrar a los logs del otro servicio.
				if statusErr.Body != "boom" {
					t.Errorf("cuerpo = %q, se esperaba conservarlo", statusErr.Body)
				}
			},
		},
		{
			name: "status 400 del cliente",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			assert: func(t *testing.T, err error) {
				var statusErr *UpstreamStatusError
				if !errors.As(err, &statusErr) || statusErr.Status != http.StatusBadRequest {
					t.Errorf("error = %v, se esperaba UpstreamStatusError con 400", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			client := NewStatsClient(server.URL, time.Second, 0, discardLogger())
			_, err := client.Compute(context.Background(), testMatrices(), "", "")

			if err == nil {
				t.Fatal("se esperaba un error")
			}
			tc.assert(t, err)
		})
	}
}

func TestComputeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()

	client := NewStatsClient(server.URL, 30*time.Millisecond, 0, discardLogger())

	_, err := client.Compute(context.Background(), testMatrices(), "", "")
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Errorf("error = %v, se esperaba ErrUpstreamTimeout", err)
	}
}

func TestComputeUnreachable(t *testing.T) {
	// Puerto cerrado: la conexión se rechaza de inmediato.
	client := NewStatsClient("http://127.0.0.1:1", time.Second, 0, discardLogger())

	_, err := client.Compute(context.Background(), testMatrices(), "", "")
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("error = %v, se esperaba ErrUpstreamUnavailable", err)
	}
}

// TestComputeRetries verifica la política: los fallos del servidor se
// reintentan porque suelen ser transitorios, y los del cliente no, porque
// repetir una petición mal formada solo gasta tiempo.
func TestComputeRetries(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		maxRetries   int
		wantAttempts int
	}{
		{"5xx se reintenta", http.StatusServiceUnavailable, 2, 3},
		{"4xx no se reintenta", http.StatusBadRequest, 3, 1},
		{"sin reintentos configurados", http.StatusInternalServerError, 0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			client := NewStatsClient(server.URL, time.Second, tc.maxRetries, discardLogger())
			_, _ = client.Compute(context.Background(), testMatrices(), "", "")

			if attempts != tc.wantAttempts {
				t.Errorf("intentos = %d, se esperaban %d", attempts, tc.wantAttempts)
			}
		})
	}
}

// TestComputeRetrySendsBodyAgain cubre un fallo silencioso clásico: reutilizar
// el lector del cuerpo hace que el reintento viaje con el cuerpo vacío.
func TestComputeRetrySendsBodyAgain(t *testing.T) {
	var lastBody []byte
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		lastBody, _ = io.ReadAll(r.Body)
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()

	client := NewStatsClient(server.URL, time.Second, 1, discardLogger())
	if _, err := client.Compute(context.Background(), testMatrices(), "", ""); err != nil {
		t.Fatalf("el reintento no tuvo éxito: %v", err)
	}

	if len(lastBody) == 0 {
		t.Fatal("el reintento envió un cuerpo vacío")
	}
	var sent struct {
		Matrices map[string][][]float64 `json:"matrices"`
	}
	if err := json.Unmarshal(lastBody, &sent); err != nil || len(sent.Matrices) == 0 {
		t.Errorf("el cuerpo del reintento no llegó completo: %s", lastBody)
	}
}

func TestComputeRespectsCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, validResponse)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // se cancela antes de empezar

	client := NewStatsClient(server.URL, time.Second, 1, discardLogger())
	if _, err := client.Compute(ctx, testMatrices(), "", ""); err == nil {
		t.Error("se esperaba un error con el contexto ya cancelado")
	}
}

func TestHealth(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"upstream sano", http.StatusOK, false},
		{"upstream degradado", http.StatusServiceUnavailable, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("ruta = %q, se esperaba /health", r.URL.Path)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			client := NewStatsClient(server.URL, time.Second, 0, discardLogger())
			err := client.Health(context.Background())

			if tc.wantErr && err == nil {
				t.Error("se esperaba un error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("error inesperado: %v", err)
			}
		})
	}
}

func TestHealthUnreachable(t *testing.T) {
	client := NewStatsClient("http://127.0.0.1:1", time.Second, 0, discardLogger())

	if err := client.Health(context.Background()); err == nil {
		t.Error("se esperaba un error con el upstream inalcanzable")
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"más corto que el límite", "abc", 10, "abc"},
		{"justo en el límite", "abcde", 5, "abcde"},
		{"se recorta", "abcdefgh", 3, "abc…"},
		{"cadena vacía", "", 5, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.input, tc.max); got != tc.want {
				t.Errorf("truncate = %q, se esperaba %q", got, tc.want)
			}
		})
	}
}
