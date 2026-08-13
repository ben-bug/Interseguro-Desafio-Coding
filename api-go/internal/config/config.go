// Package config centraliza la lectura de la configuración del servicio desde
// variables de entorno.
//
// Toda la configuración se resuelve una sola vez al arrancar y se valida de
// inmediato: si algo falta o es inválido el proceso no levanta. Es preferible
// fallar al iniciar, cuando el problema es evidente, que descubrirlo en el
// primer request en producción.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config agrupa todos los parámetros de ejecución de la API Go.
type Config struct {
	// Port es el puerto TCP en que escucha el servidor.
	Port string
	// StatsAPIURL es la URL base de la API Node de estadísticas.
	StatsAPIURL string
	// StatsTimeout acota cada intento de llamada a la API de estadísticas.
	StatsTimeout time.Duration
	// StatsMaxRetries es la cantidad de reintentos tras el primer intento.
	StatsMaxRetries int
	// MaxMatrixDimension limita filas y columnas de la matriz de entrada.
	MaxMatrixDimension int
	// JWTSecret es el secreto HS256 compartido con la API de estadísticas.
	JWTSecret string
	// JWTIssuer y JWTAudience se emiten y validan como claims `iss` y `aud`.
	JWTIssuer   string
	JWTAudience string
	// JWTTTL es la vigencia del token emitido.
	JWTTTL time.Duration
	// DemoUsername y DemoPassword son las credenciales aceptadas por el
	// endpoint de login. Reemplazan a una base de usuarios real, que está
	// fuera del alcance del desafío.
	DemoUsername string
	DemoPassword string
}

// Load construye la configuración desde el entorno, aplicando los valores por
// defecto documentados en .env.example.
func Load() (Config, error) {
	cfg := Config{
		Port:               envString("GO_API_PORT", "8080"),
		StatsAPIURL:        envString("STATS_API_URL", "http://localhost:3000"),
		StatsTimeout:       time.Duration(envInt("STATS_API_TIMEOUT_SECONDS", 5)) * time.Second,
		StatsMaxRetries:    envInt("STATS_API_MAX_RETRIES", 1),
		MaxMatrixDimension: envInt("MAX_MATRIX_DIMENSION", 256),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTIssuer:          envString("JWT_ISSUER", "interseguro-qr-api"),
		JWTAudience:        envString("JWT_AUDIENCE", "interseguro-clients"),
		JWTTTL:             time.Duration(envInt("JWT_TTL_MINUTES", 15)) * time.Minute,
		DemoUsername:       envString("DEMO_USERNAME", "demo"),
		DemoPassword:       os.Getenv("DEMO_PASSWORD"),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validate rechaza combinaciones que dejarían al servicio en un estado
// inseguro o inoperable.
func (c Config) validate() error {
	// Sin secreto no hay forma de firmar ni verificar tokens. Generar uno al
	// vuelo sería peor: cada réplica del servicio firmaría con un secreto
	// distinto y los tokens dejarían de ser válidos entre instancias.
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET es obligatorio: definirlo en el entorno (ver .env.example)")
	}
	if c.DemoPassword == "" {
		return errors.New("DEMO_PASSWORD es obligatorio: definirlo en el entorno (ver .env.example)")
	}
	if c.StatsAPIURL == "" {
		return errors.New("STATS_API_URL es obligatorio")
	}
	if c.MaxMatrixDimension < 1 {
		return fmt.Errorf("MAX_MATRIX_DIMENSION debe ser positivo, se recibió %d", c.MaxMatrixDimension)
	}
	if c.StatsMaxRetries < 0 {
		return fmt.Errorf("STATS_API_MAX_RETRIES no puede ser negativo, se recibió %d", c.StatsMaxRetries)
	}
	if c.StatsTimeout <= 0 {
		return errors.New("STATS_API_TIMEOUT_SECONDS debe ser positivo")
	}
	if c.JWTTTL <= 0 {
		return errors.New("JWT_TTL_MINUTES debe ser positivo")
	}
	return nil
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt devuelve el valor por defecto si la variable está ausente o no es un
// entero válido. Un valor mal escrito no debe tumbar el arranque por sí solo:
// validate() se encarga después de rechazar los rangos imposibles.
func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
