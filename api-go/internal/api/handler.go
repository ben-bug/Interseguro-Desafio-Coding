package api

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/socius/interseguro-challenge/api-go/internal/auth"
	"github.com/socius/interseguro-challenge/api-go/internal/client"
	"github.com/socius/interseguro-challenge/api-go/internal/config"
	"github.com/socius/interseguro-challenge/api-go/internal/matrix"
)

// Version se sobrescribe en tiempo de compilación con -ldflags. Permite saber
// qué build está corriendo sin entrar al contenedor.
var Version = "dev"

// Handler agrupa las dependencias de los endpoints.
type Handler struct {
	cfg    config.Config
	stats  *client.StatsClient
	auth   *auth.Manager
	logger *slog.Logger
}

// NewHandler construye el handler con sus dependencias ya resueltas. Se
// inyectan en vez de construirse acá para poder sustituirlas en los tests.
func NewHandler(cfg config.Config, stats *client.StatsClient, authManager *auth.Manager, logger *slog.Logger) *Handler {
	return &Handler{cfg: cfg, stats: stats, auth: authManager, logger: logger}
}

// Login valida las credenciales y emite un JWT.
func (h *Handler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"el cuerpo debe ser un JSON con los campos 'username' y 'password'", nil)
	}

	// Comparación en tiempo constante: comparar con == permitiría deducir el
	// prefijo correcto de la contraseña midiendo el tiempo de respuesta. Ambas
	// comparaciones se evalúan siempre, sin cortocircuito.
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.cfg.DemoUsername)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.cfg.DemoPassword)) == 1
	if !userOK || !passOK {
		// El mensaje no distingue entre usuario inexistente y contraseña
		// incorrecta: hacerlo permitiría enumerar usuarios válidos.
		return NewAPIError(http.StatusUnauthorized, CodeInvalidCredentials,
			"usuario o contraseña incorrectos", nil)
	}

	token, expiresAt, err := h.auth.Issue(req.Username)
	if err != nil {
		h.logger.ErrorContext(c.Context(), "no se pudo emitir el token", slog.Any("error", err))
		return NewAPIError(http.StatusInternalServerError, CodeInternal, "no se pudo emitir el token", nil)
	}

	return c.JSON(LoginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		ExpiresIn: int(h.auth.TTL().Seconds()),
	})
}

// QR factoriza la matriz recibida y le adjunta las estadísticas calculadas por
// la API Node.
func (h *Handler) QR(c fiber.Ctx) error {
	var req QRRequest
	if err := c.Bind().JSON(&req); err != nil {
		return NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"el cuerpo debe ser un JSON con el campo 'matrix' como array de arrays de números", nil)
	}
	if req.Matrix == nil {
		return NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"falta el campo 'matrix' en el cuerpo del request", nil)
	}

	if verr := matrix.Validate(req.Matrix, h.cfg.MaxMatrixDimension); verr != nil {
		return NewAPIError(http.StatusBadRequest, string(verr.Code), verr.Message, verr.Details)
	}

	mode, err := parseMode(c.Query("mode"))
	if err != nil {
		return err
	}

	start := time.Now()
	decomposition := matrix.QR(req.Matrix, mode)
	elapsed := time.Since(start)

	response := QRResponse{
		Q: decomposition.Q,
		R: decomposition.R,
		Meta: QRMeta{
			Rows:       req.Matrix.Rows(),
			Cols:       req.Matrix.Cols(),
			Mode:       string(decomposition.Mode),
			Algorithm:  "householder",
			Residual:   decomposition.Residual,
			DurationMs: float64(elapsed.Microseconds()) / 1000,
			RequestID:  requestid.FromContext(c),
		},
	}

	// withStats=false permite ejercitar la API Go de forma aislada, lo que
	// resulta útil para diagnosticar si un fallo viene de este servicio o del
	// upstream.
	if c.Query("withStats") == "false" {
		return c.JSON(response)
	}

	stats, err := h.stats.Compute(
		c.Context(),
		map[string]matrix.Matrix{"q": decomposition.Q, "r": decomposition.R},
		c.Get(fiber.HeaderAuthorization),
		requestid.FromContext(c),
	)
	if err != nil {
		return h.mapUpstreamError(err)
	}

	response.Statistics = stats
	return c.JSON(response)
}

// Rotate devuelve la matriz rotada 90° en sentido horario.
func (h *Handler) Rotate(c fiber.Ctx) error {
	var req RotateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"el cuerpo debe ser un JSON con el campo 'matrix' como array de arrays de números", nil)
	}
	if req.Matrix == nil {
		return NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"falta el campo 'matrix' en el cuerpo del request", nil)
	}
	if verr := matrix.Validate(req.Matrix, h.cfg.MaxMatrixDimension); verr != nil {
		return NewAPIError(http.StatusBadRequest, string(verr.Code), verr.Message, verr.Details)
	}

	rotated := matrix.Rotate90(req.Matrix)

	return c.JSON(RotateResponse{
		Rotated: rotated,
		Meta: RotateMeta{
			Rows:      rotated.Rows(),
			Cols:      rotated.Cols(),
			Direction: "clockwise",
			Degrees:   90,
			RequestID: requestid.FromContext(c),
		},
	})
}

// Health es el chequeo de vitalidad (liveness). No consulta dependencias
// externas a propósito: si lo hiciera, una caída de la API de estadísticas
// haría que el orquestador reiniciara este servicio, que está perfectamente
// sano, en lugar de aislar el problema donde ocurre.
func (h *Handler) Health(c fiber.Ctx) error {
	return c.JSON(HealthResponse{Status: "ok", Service: "qr-api-go", Version: Version})
}

// Ready es el chequeo de disponibilidad (readiness): incluye el upstream,
// porque sin él este servicio no puede completar su función principal.
func (h *Handler) Ready(c fiber.Ctx) error {
	if err := h.stats.Health(c.Context()); err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(HealthResponse{
			Status: "degraded", Service: "qr-api-go", Version: Version, Upstream: "unreachable",
		})
	}
	return c.JSON(HealthResponse{Status: "ok", Service: "qr-api-go", Version: Version, Upstream: "ok"})
}

// parseMode traduce el parámetro `mode` de la query.
func parseMode(raw string) (matrix.Mode, error) {
	switch raw {
	case "", string(matrix.ModeFull):
		return matrix.ModeFull, nil
	case string(matrix.ModeReduced):
		return matrix.ModeReduced, nil
	default:
		return "", NewAPIError(http.StatusBadRequest, CodeInvalidBody,
			"el parámetro 'mode' solo admite los valores 'full' o 'reduced'",
			map[string]any{"received": raw, "allowed": []string{"full", "reduced"}})
	}
}

// mapUpstreamError traduce los fallos del cliente de estadísticas a errores
// HTTP, distinguiendo con qué status debe responderse cada caso.
func (h *Handler) mapUpstreamError(err error) error {
	switch {
	case errors.Is(err, client.ErrUpstreamTimeout):
		return NewAPIError(http.StatusGatewayTimeout, CodeUpstreamTimeout,
			"la API de estadísticas no respondió dentro del tiempo permitido", nil)

	case errors.Is(err, client.ErrUpstreamUnavailable):
		return NewAPIError(http.StatusBadGateway, CodeUpstreamUnavailable,
			"la API de estadísticas no está disponible", nil)
	}

	var statusErr *client.UpstreamStatusError
	if errors.As(err, &statusErr) {
		// Un 401 del upstream con el mismo token que este servicio ya validó
		// apunta a un desajuste de configuración entre ambos (típicamente
		// JWT_SECRET distinto), no a un problema del cliente.
		if statusErr.Status == http.StatusUnauthorized || statusErr.Status == http.StatusForbidden {
			return NewAPIError(http.StatusBadGateway, CodeUpstreamError,
				"la API de estadísticas rechazó la autenticación: revisar que ambos servicios compartan JWT_SECRET",
				map[string]any{"upstreamStatus": statusErr.Status})
		}
		return NewAPIError(http.StatusBadGateway, CodeUpstreamError,
			"la API de estadísticas devolvió una respuesta inesperada",
			map[string]any{"upstreamStatus": statusErr.Status})
	}

	return NewAPIError(http.StatusBadGateway, CodeUpstreamError,
		"no se pudieron obtener las estadísticas", nil)
}
