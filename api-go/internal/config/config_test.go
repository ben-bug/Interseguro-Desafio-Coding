package config

import (
	"testing"
	"time"
)

// setEnv aplica las variables dadas y las restaura al terminar el subtest.
// t.Setenv se encarga de la restauración e impide que el test corra en paralelo,
// que es justo lo que se necesita al manipular estado global del proceso.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	// Las claves obligatorias se limpian primero para que cada caso parta de un
	// entorno conocido, sin heredar lo que haya definido el shell.
	for _, key := range []string{
		"GO_API_PORT", "STATS_API_URL", "STATS_API_TIMEOUT_SECONDS", "STATS_API_MAX_RETRIES",
		"MAX_MATRIX_DIMENSION", "JWT_SECRET", "JWT_ISSUER", "JWT_AUDIENCE", "JWT_TTL_MINUTES",
		"DEMO_USERNAME", "DEMO_PASSWORD",
	} {
		t.Setenv(key, "")
	}
	for key, value := range vars {
		t.Setenv(key, value)
	}
}

// validEnv es el conjunto mínimo con el que Load debe tener éxito.
func validEnv() map[string]string {
	return map[string]string{
		"JWT_SECRET":    "secreto-de-prueba",
		"DEMO_PASSWORD": "clave-de-prueba",
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load devolvió error: %v", err)
	}

	checks := []struct {
		name      string
		got, want any
	}{
		{"Port", cfg.Port, "8080"},
		{"StatsAPIURL", cfg.StatsAPIURL, "http://localhost:3000"},
		{"StatsTimeout", cfg.StatsTimeout, 5 * time.Second},
		{"StatsMaxRetries", cfg.StatsMaxRetries, 1},
		{"MaxMatrixDimension", cfg.MaxMatrixDimension, 256},
		{"JWTIssuer", cfg.JWTIssuer, "interseguro-qr-api"},
		{"JWTAudience", cfg.JWTAudience, "interseguro-clients"},
		{"JWTTTL", cfg.JWTTTL, 15 * time.Minute},
		{"DemoUsername", cfg.DemoUsername, "demo"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, se esperaba %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadOverridesFromEnv(t *testing.T) {
	env := validEnv()
	env["GO_API_PORT"] = "9090"
	env["STATS_API_URL"] = "http://api-node:3000"
	env["STATS_API_TIMEOUT_SECONDS"] = "12"
	env["MAX_MATRIX_DIMENSION"] = "64"
	env["JWT_TTL_MINUTES"] = "60"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load devolvió error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, se esperaba \"9090\"", cfg.Port)
	}
	if cfg.StatsAPIURL != "http://api-node:3000" {
		t.Errorf("StatsAPIURL = %q", cfg.StatsAPIURL)
	}
	if cfg.StatsTimeout != 12*time.Second {
		t.Errorf("StatsTimeout = %v, se esperaba 12s", cfg.StatsTimeout)
	}
	if cfg.MaxMatrixDimension != 64 {
		t.Errorf("MaxMatrixDimension = %d, se esperaba 64", cfg.MaxMatrixDimension)
	}
	if cfg.JWTTTL != time.Hour {
		t.Errorf("JWTTTL = %v, se esperaba 1h", cfg.JWTTTL)
	}
}

func TestLoadTrimsStringEnvironmentValues(t *testing.T) {
	env := validEnv()
	env["GO_API_PORT"] = " 9090 "
	env["STATS_API_URL"] = "  http://localhost:3000/  "
	env["DEMO_USERNAME"] = " demo "
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load devolviÃ³ error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, se esperaba 9090", cfg.Port)
	}
	if cfg.StatsAPIURL != "http://localhost:3000/" {
		t.Errorf("StatsAPIURL = %q", cfg.StatsAPIURL)
	}
	if cfg.DemoUsername != "demo" {
		t.Errorf("DemoUsername = %q", cfg.DemoUsername)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(map[string]string)
		wantFail bool
	}{
		{
			name:     "sin JWT_SECRET",
			mutate:   func(e map[string]string) { delete(e, "JWT_SECRET") },
			wantFail: true,
		},
		{
			name:     "sin DEMO_PASSWORD",
			mutate:   func(e map[string]string) { delete(e, "DEMO_PASSWORD") },
			wantFail: true,
		},
		{
			name:     "URL de estadísticas inválida",
			mutate:   func(e map[string]string) { e["STATS_API_URL"] = "localhost:3000" },
			wantFail: true,
		},
		{
			name:     "dimensión máxima en cero",
			mutate:   func(e map[string]string) { e["MAX_MATRIX_DIMENSION"] = "0" },
			wantFail: true,
		},
		{
			name:     "reintentos negativos",
			mutate:   func(e map[string]string) { e["STATS_API_MAX_RETRIES"] = "-1" },
			wantFail: true,
		},
		{
			name:     "timeout en cero",
			mutate:   func(e map[string]string) { e["STATS_API_TIMEOUT_SECONDS"] = "0" },
			wantFail: true,
		},
		{
			// Un valor no numérico cae al default documentado en lugar de
			// impedir el arranque: es un error de tipeo recuperable.
			name:     "timeout no numérico usa el valor por defecto",
			mutate:   func(e map[string]string) { e["STATS_API_TIMEOUT_SECONDS"] = "cinco" },
			wantFail: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			tc.mutate(env)
			setEnv(t, env)

			_, err := Load()
			if tc.wantFail && err == nil {
				t.Error("se esperaba un error de configuración, Load tuvo éxito")
			}
			if !tc.wantFail && err != nil {
				t.Errorf("Load devolvió error inesperado: %v", err)
			}
		})
	}
}
