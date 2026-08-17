// Package client implementa el consumo de la API Node de estadísticas.
//
// Es el único punto del servicio que conoce el contrato del upstream: si ese
// contrato cambia, solo hay que tocar este paquete.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/socius/interseguro-challenge/api-go/internal/matrix"
)

var (
	// ErrUpstreamTimeout indica que la API de estadísticas no respondió dentro
	// del plazo configurado.
	ErrUpstreamTimeout = errors.New("la API de estadísticas no respondió a tiempo")
	// ErrUpstreamUnavailable indica que no se pudo establecer la comunicación
	// (DNS, conexión rechazada, servicio caído).
	ErrUpstreamUnavailable = errors.New("la API de estadísticas no está disponible")
)

// UpstreamStatusError representa una respuesta HTTP no exitosa del upstream.
// Conserva el status y el cuerpo para poder diagnosticar sin revisar los logs
// del otro servicio.
type UpstreamStatusError struct {
	Status int
	Body   string
}

func (e *UpstreamStatusError) Error() string {
	return fmt.Sprintf("la API de estadísticas respondió %d: %s", e.Status, e.Body)
}

// MatrixStats son las estadísticas de una matriz individual.
type MatrixStats struct {
	Max        float64 `json:"max"`
	Min        float64 `json:"min"`
	Average    float64 `json:"average"`
	Sum        float64 `json:"sum"`
	Count      int     `json:"count"`
	Rows       int     `json:"rows"`
	Cols       int     `json:"cols"`
	IsSquare   bool    `json:"isSquare"`
	IsDiagonal bool    `json:"isDiagonal"`
	// Tolerance es el umbral con que se evaluó IsDiagonal. Se deriva de la
	// magnitud de cada matriz por separado, de modo que difiere entre Q y R.
	Tolerance float64 `json:"tolerance"`
}

// OverallStats son las estadísticas agregadas sobre todas las matrices.
type OverallStats struct {
	Max     float64 `json:"max"`
	Min     float64 `json:"min"`
	Average float64 `json:"average"`
	Sum     float64 `json:"sum"`
	Count   int     `json:"count"`
}

// StatisticsResponse es la respuesta de la API Node.
type StatisticsResponse struct {
	Overall     OverallStats           `json:"overall"`
	PerMatrix   map[string]MatrixStats `json:"perMatrix"`
	AnyDiagonal bool                   `json:"anyDiagonal"`
	// ToleranceFactor es el factor relativo del que se deriva la tolerancia de
	// cada matriz; el umbral concreto va en cada entrada de PerMatrix.
	ToleranceFactor float64 `json:"toleranceFactor"`
}

// statisticsRequest es el cuerpo que se envía al upstream.
type statisticsRequest struct {
	Matrices map[string]matrix.Matrix `json:"matrices"`
}

// StatsClient consume la API de estadísticas con timeout y reintentos.
type StatsClient struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	logger     *slog.Logger
}

// NewStatsClient construye el cliente. El timeout se aplica a cada intento por
// separado, no al conjunto: el plazo total lo gobierna el contexto del request
// entrante, que se propaga hasta acá.
func NewStatsClient(baseURL string, timeout time.Duration, maxRetries int, logger *slog.Logger) *StatsClient {
	return &StatsClient{
		// TrimRight y no TrimSuffix: este último quita una sola barra, y una URL
		// copiada a mano puede traer varias. Cada barra sobrante produciría
		// rutas como `…:3000//api/v1/statistics`, que algunos servidores
		// rechazan con 404. El recorte de espacios cubre el caso de una variable
		// de entorno con un salto de línea o un espacio al final.
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				// Reutilizar conexiones importa: en el camino caliente se hace
				// una llamada al upstream por cada request entrante.
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// Compute envía las matrices al upstream y devuelve las estadísticas.
//
// authHeader y requestID se propagan tal cual: el primero porque la API de
// estadísticas exige el mismo JWT del usuario final, y el segundo para poder
// correlacionar en los logs una traza que atraviesa los dos servicios.
func (c *StatsClient) Compute(
	ctx context.Context,
	matrices map[string]matrix.Matrix,
	authHeader, requestID string,
) (*StatisticsResponse, error) {
	body, err := json.Marshal(statisticsRequest{Matrices: matrices})
	if err != nil {
		return nil, fmt.Errorf("serializando las matrices: %w", err)
	}

	url := c.baseURL + "/api/v1/statistics"
	var lastErr error

	// Un intento inicial más maxRetries reintentos.
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff exponencial: 200 ms, 400 ms, 800 ms…
			delay := time.Duration(200*(1<<(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ErrUpstreamTimeout
			case <-time.After(delay):
			}
			c.logger.WarnContext(ctx, "reintentando la llamada a la API de estadísticas",
				slog.Int("attempt", attempt), slog.String("requestId", requestID), slog.Any("error", lastErr))
		}

		stats, err := c.doRequest(ctx, url, body, authHeader, requestID)
		if err == nil {
			return stats, nil
		}
		lastErr = err

		// Un 4xx es un problema del request, no del upstream: reintentarlo
		// solo repetiría el mismo fallo y gastaría el presupuesto de tiempo.
		var statusErr *UpstreamStatusError
		if errors.As(err, &statusErr) && statusErr.Status < 500 {
			return nil, err
		}
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
	}

	return nil, lastErr
}

// Health consulta el endpoint de salud del upstream. Lo usa el chequeo de
// readiness: sin la API de estadísticas disponible, este servicio puede
// responder pero no completar su función principal.
func (c *StatsClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("construyendo el request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return ErrUpstreamTimeout
		}
		return fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	// El cuerpo se descarta pero se drena, para que la conexión pueda volver
	// al pool de keep-alive en lugar de cerrarse.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	if resp.StatusCode != http.StatusOK {
		return &UpstreamStatusError{Status: resp.StatusCode}
	}
	return nil
}

// doRequest ejecuta un único intento.
func (c *StatsClient) doRequest(
	ctx context.Context,
	url string,
	body []byte,
	authHeader, requestID string,
) (*StatisticsResponse, error) {
	// El cuerpo se envuelve en un lector nuevo en cada intento: un reintento
	// sobre un lector ya consumido enviaría un cuerpo vacío.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("construyendo el request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, ErrUpstreamTimeout
		}
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	// Se acota la lectura: un upstream comprometido o con un bug no debe poder
	// agotar la memoria de este proceso con una respuesta gigante.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: leyendo la respuesta: %v", ErrUpstreamUnavailable, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamStatusError{Status: resp.StatusCode, Body: truncate(string(raw), 512)}
	}

	var stats StatisticsResponse
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, fmt.Errorf("%w: respuesta ilegible: %v", ErrUpstreamUnavailable, err)
	}
	return &stats, nil
}

// isTimeout detecta los timeouts de red, que no siempre se envuelven como
// context.DeadlineExceeded (el Timeout del http.Client produce su propio tipo).
func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
