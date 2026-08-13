package api

import (
	"time"

	"github.com/socius/interseguro-challenge/api-go/internal/client"
	"github.com/socius/interseguro-challenge/api-go/internal/matrix"
)

// QRRequest es el cuerpo de POST /api/v1/qr.
type QRRequest struct {
	// Matrix es la matriz rectangular de entrada, como array de arrays.
	Matrix matrix.Matrix `json:"matrix"`
}

// QRMeta acompaña al resultado con información de trazabilidad y calidad
// numérica. Residual permite al consumidor comprobar por sí mismo que la
// factorización reconstruye la matriz original, sin tener que confiar a ciegas
// en el servicio.
type QRMeta struct {
	Rows       int     `json:"rows"`
	Cols       int     `json:"cols"`
	Mode       string  `json:"mode"`
	Algorithm  string  `json:"algorithm"`
	Residual   float64 `json:"residual"`
	DurationMs float64 `json:"durationMs"`
	RequestID  string  `json:"requestId,omitempty"`
}

// QRResponse es la respuesta de POST /api/v1/qr.
type QRResponse struct {
	Q    matrix.Matrix `json:"q"`
	R    matrix.Matrix `json:"r"`
	Meta QRMeta        `json:"meta"`
	// Statistics viene de la API Node. Es nil cuando se invoca con
	// ?withStats=false, útil para aislar fallos entre ambos servicios.
	Statistics *client.StatisticsResponse `json:"statistics,omitempty"`
}

// RotateRequest es el cuerpo de POST /api/v1/rotate.
type RotateRequest struct {
	Matrix matrix.Matrix `json:"matrix"`
}

// RotateResponse es la respuesta de POST /api/v1/rotate.
type RotateResponse struct {
	Rotated matrix.Matrix `json:"rotated"`
	Meta    RotateMeta    `json:"meta"`
}

// RotateMeta describe la transformación aplicada.
type RotateMeta struct {
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
	Direction string `json:"direction"`
	Degrees   int    `json:"degrees"`
	RequestID string `json:"requestId,omitempty"`
}

// LoginRequest es el cuerpo de POST /api/v1/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse entrega el token y su vencimiento, para que el cliente pueda
// renovarlo antes de que caduque en lugar de esperar el primer 401.
type LoginResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"tokenType"`
	ExpiresAt time.Time `json:"expiresAt"`
	ExpiresIn int       `json:"expiresIn"`
}

// HealthResponse es la respuesta de los endpoints de salud.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
	// Upstream solo aparece en el chequeo de readiness.
	Upstream string `json:"upstream,omitempty"`
}
