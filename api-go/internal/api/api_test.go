package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/socius/interseguro-challenge/api-go/internal/config"
)

const (
	testSecret   = "secreto-de-prueba"
	testUser     = "demo"
	testPassword = "clave-de-prueba"
)

// discardLogger evita ensuciar la salida del test con las líneas de request.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func testConfig(statsURL string) config.Config {
	return config.Config{
		Port:               "0",
		StatsAPIURL:        statsURL,
		StatsTimeout:       2 * time.Second,
		StatsMaxRetries:    0, // sin reintentos: los tests deben ser deterministas y rápidos
		MaxMatrixDimension: 8,
		JWTSecret:          testSecret,
		JWTIssuer:          "test-issuer",
		JWTAudience:        "test-audience",
		JWTTTL:             15 * time.Minute,
		DemoUsername:       testUser,
		DemoPassword:       testPassword,
	}
}

// fakeStatsResponse reproduce literalmente la forma que devuelve la API Node.
//
// Es una copia de una respuesta real del servicio, no una aproximación escrita a
// mano: un stub que solo se parezca al contrato deja pasar los desajustes entre
// ambos servicios, que es justo lo que estos tests deben detectar.
const fakeStatsResponse = `{
  "overall": {"max": 10, "min": -2, "average": 3.5, "sum": 42, "count": 12},
  "perMatrix": {
    "q": {"max": 1, "min": -1, "average": 0, "sum": 0, "count": 4, "rows": 2, "cols": 2, "isSquare": true, "isDiagonal": true, "tolerance": 1e-9},
    "r": {"max": 10, "min": 0, "average": 5, "sum": 20, "count": 4, "rows": 2, "cols": 2, "isSquare": true, "isDiagonal": false, "tolerance": 1e-8}
  },
  "anyDiagonal": true,
  "toleranceFactor": 1e-9
}`

// capturedRequest guarda lo que el upstream recibió, para poder afirmar sobre
// la propagación de encabezados y el cuerpo enviado.
type capturedRequest struct {
	authorization string
	requestID     string
	body          []byte
	calls         int
}

// newStatsStub levanta un upstream simulado. handler puede ser nil para usar la
// respuesta exitosa por defecto.
func newStatsStub(t *testing.T, captured *capturedRequest, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			captured.calls++
			captured.authorization = r.Header.Get("Authorization")
			captured.requestID = r.Header.Get("X-Request-ID")
			captured.body, _ = io.ReadAll(r.Body)
		}
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fakeStatsResponse)
	}))
	t.Cleanup(server.Close)
	return server
}

// login obtiene un token válido a través del endpoint real, en lugar de firmar
// uno a mano: así el test también cubre que ambos extremos usen el mismo
// emisor, audiencia y secreto.
func login(t *testing.T, app *fiber.App) string {
	t.Helper()

	resp := doRequest(t, app, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": testUser, "password": testPassword})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login falló con status %d", resp.StatusCode)
	}

	var body LoginResponse
	decodeBody(t, resp, &body)
	return body.Token
}

// doRequest ejecuta un request contra la app en memoria, sin abrir un puerto.
func doRequest(t *testing.T, app *fiber.App, method, path, token string, payload any) *http.Response {
	t.Helper()

	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("no se pudo serializar el payload: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, path, reader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// El timeout por defecto de app.Test es 1 s, insuficiente para los casos
	// que ejercitan deliberadamente un upstream lento.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatalf("app.Test devolvió error: %v", err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("no se pudo decodificar la respuesta: %v", err)
	}
}

func assertErrorCode(t *testing.T, resp *http.Response, wantStatus int, wantCode string) {
	t.Helper()

	var body ErrorResponse
	decodeBody(t, resp, &body)

	if resp.StatusCode != wantStatus {
		t.Errorf("status = %d, se esperaba %d (código %s)", resp.StatusCode, wantStatus, body.Error.Code)
	}
	if body.Error.Code != wantCode {
		t.Errorf("código = %q, se esperaba %q", body.Error.Code, wantCode)
	}
	if body.Error.Message == "" {
		t.Error("el error no trae mensaje legible")
	}
}

// --- Autenticación ---------------------------------------------------------

func TestLogin(t *testing.T) {
	app := NewApp(testConfig("http://unused"), discardLogger())

	cases := []struct {
		name       string
		payload    any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "credenciales válidas",
			payload:    map[string]string{"username": testUser, "password": testPassword},
			wantStatus: http.StatusOK,
		},
		{
			name:       "contraseña incorrecta",
			payload:    map[string]string{"username": testUser, "password": "incorrecta"},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeInvalidCredentials,
		},
		{
			name:       "usuario inexistente",
			payload:    map[string]string{"username": "fantasma", "password": testPassword},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeInvalidCredentials,
		},
		{
			name:       "cuerpo vacío",
			payload:    map[string]string{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeInvalidCredentials,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(t, app, http.MethodPost, "/api/v1/auth/login", "", tc.payload)

			if tc.wantStatus != http.StatusOK {
				assertErrorCode(t, resp, tc.wantStatus, tc.wantCode)
				return
			}

			var body LoginResponse
			decodeBody(t, resp, &body)
			if body.Token == "" {
				t.Error("no se devolvió token")
			}
			if body.TokenType != "Bearer" {
				t.Errorf("tokenType = %q, se esperaba \"Bearer\"", body.TokenType)
			}
			if body.ExpiresIn != 900 {
				t.Errorf("expiresIn = %d, se esperaban 900 segundos", body.ExpiresIn)
			}
		})
	}
}

func TestProtectedEndpointsRequireToken(t *testing.T) {
	app := NewApp(testConfig("http://unused"), discardLogger())
	payload := map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}}

	cases := []struct {
		name   string
		header string
	}{
		{"sin encabezado", ""},
		{"esquema incorrecto", "Basic abc123"},
		{"token vacío", "Bearer "},
		{"token inventado", "Bearer no-es-un-jwt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/qr", bytes.NewReader(encoded))
			req.Header.Set("Content-Type", "application/json")
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test devolvió error: %v", err)
			}
			assertErrorCode(t, resp, http.StatusUnauthorized, CodeUnauthorized)
		})
	}
}

func TestExpiredTokenIsReported(t *testing.T) {
	cfg := testConfig("http://unused")
	cfg.JWTTTL = -time.Minute // el token nace vencido
	app := NewApp(cfg, discardLogger())

	resp := doRequest(t, app, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": testUser, "password": testPassword})
	var loginBody LoginResponse
	decodeBody(t, resp, &loginBody)

	resp = doRequest(t, app, http.MethodPost, "/api/v1/qr", loginBody.Token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	assertErrorCode(t, resp, http.StatusUnauthorized, CodeTokenExpired)
}

// --- Endpoint QR -----------------------------------------------------------

func TestQRSuccess(t *testing.T) {
	captured := &capturedRequest{}
	stub := newStatsStub(t, captured, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{12, -51, 4}, {6, 167, -68}, {-4, 24, -41}}})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", resp.StatusCode)
	}

	var body QRResponse
	decodeBody(t, resp, &body)

	if body.Q.Rows() != 3 || body.Q.Cols() != 3 {
		t.Errorf("Q es %d×%d, se esperaba 3×3", body.Q.Rows(), body.Q.Cols())
	}
	if body.R.Rows() != 3 || body.R.Cols() != 3 {
		t.Errorf("R es %d×%d, se esperaba 3×3", body.R.Rows(), body.R.Cols())
	}
	if body.Meta.Algorithm != "householder" {
		t.Errorf("algorithm = %q", body.Meta.Algorithm)
	}
	if body.Meta.Mode != "full" {
		t.Errorf("mode = %q, se esperaba \"full\"", body.Meta.Mode)
	}
	if body.Meta.Residual > 1e-10 {
		t.Errorf("residual = %g: la factorización no reconstruye la matriz", body.Meta.Residual)
	}
	if body.Meta.RequestID == "" {
		t.Error("meta no trae requestId")
	}
	if body.Statistics == nil {
		t.Fatal("no se adjuntaron las estadísticas del upstream")
	}
	if body.Statistics.Overall.Sum != 42 {
		t.Errorf("sum = %g, se esperaba el valor del upstream simulado (42)", body.Statistics.Overall.Sum)
	}
}

// TestStatisticsContractIsFullyDecoded verifica que la estructura Go cubra todos
// los campos que emite la API Node.
//
// Existe porque un desajuste de este tipo no rompe nada de forma visible: los
// campos que Go no declara se descartan en silencio al deserializar, y el
// cliente recibe ceros donde debería haber datos. Solo se detecta comparando el
// contrato campo por campo.
func TestStatisticsContractIsFullyDecoded(t *testing.T) {
	stub := newStatsStub(t, nil, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	var body QRResponse
	decodeBody(t, resp, &body)

	stats := body.Statistics
	if stats == nil {
		t.Fatal("no se adjuntaron las estadísticas")
	}
	if stats.ToleranceFactor != 1e-9 {
		t.Errorf("toleranceFactor = %g, se esperaba 1e-9: el campo no se está deserializando",
			stats.ToleranceFactor)
	}
	if !stats.AnyDiagonal {
		t.Error("anyDiagonal = false, el upstream simulado devuelve true")
	}

	q, ok := stats.PerMatrix["q"]
	if !ok {
		t.Fatal("falta la matriz 'q' en perMatrix")
	}
	if q.Tolerance != 1e-9 {
		t.Errorf("perMatrix.q.tolerance = %g, se esperaba 1e-9: el campo no se está deserializando",
			q.Tolerance)
	}
	if !q.IsSquare || !q.IsDiagonal {
		t.Errorf("perMatrix.q = %+v: los booleanos no se están deserializando", q)
	}
	if q.Rows != 2 || q.Cols != 2 || q.Count != 4 {
		t.Errorf("perMatrix.q dimensiones = %d×%d (count %d), se esperaba 2×2 (4)", q.Rows, q.Cols, q.Count)
	}
}

// TestQRPropagatesHeadersUpstream verifica el contrato entre servicios: la API
// Node exige el mismo JWT y usa X-Request-ID para correlacionar sus logs con
// los de este servicio.
func TestQRPropagatesHeadersUpstream(t *testing.T) {
	captured := &capturedRequest{}
	stub := newStatsStub(t, captured, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	if captured.calls != 1 {
		t.Fatalf("el upstream recibió %d llamadas, se esperaba 1", captured.calls)
	}
	if captured.authorization != "Bearer "+token {
		t.Errorf("Authorization propagado = %q, se esperaba el token del cliente", captured.authorization)
	}
	if captured.requestID == "" {
		t.Error("no se propagó X-Request-ID")
	}

	// El cuerpo debe llevar ambas matrices bajo las claves acordadas.
	var sent struct {
		Matrices map[string][][]float64 `json:"matrices"`
	}
	if err := json.Unmarshal(captured.body, &sent); err != nil {
		t.Fatalf("el upstream recibió un cuerpo ilegible: %v", err)
	}
	for _, key := range []string{"q", "r"} {
		if _, ok := sent.Matrices[key]; !ok {
			t.Errorf("falta la matriz %q en el cuerpo enviado al upstream", key)
		}
	}
}

func TestQRWithStatsDisabled(t *testing.T) {
	captured := &capturedRequest{}
	stub := newStatsStub(t, captured, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr?withStats=false", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", resp.StatusCode)
	}

	var body QRResponse
	decodeBody(t, resp, &body)
	if body.Statistics != nil {
		t.Error("se adjuntaron estadísticas pese a withStats=false")
	}
	if captured.calls != 0 {
		t.Errorf("el upstream fue invocado %d veces, no debía invocarse", captured.calls)
	}
}

func TestQRReducedMode(t *testing.T) {
	stub := newStatsStub(t, nil, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr?mode=reduced", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}, {5, 6}, {7, 8}}})

	var body QRResponse
	decodeBody(t, resp, &body)

	if body.Meta.Mode != "reduced" {
		t.Errorf("mode = %q, se esperaba \"reduced\"", body.Meta.Mode)
	}
	// La variante reducida recorta Q de 4×4 a 4×2 y R de 4×2 a 2×2.
	if body.Q.Rows() != 4 || body.Q.Cols() != 2 {
		t.Errorf("Q es %d×%d, se esperaba 4×2", body.Q.Rows(), body.Q.Cols())
	}
	if body.R.Rows() != 2 || body.R.Cols() != 2 {
		t.Errorf("R es %d×%d, se esperaba 2×2", body.R.Rows(), body.R.Cols())
	}
}

func TestQRRejectsInvalidInput(t *testing.T) {
	stub := newStatsStub(t, nil, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	cases := []struct {
		name     string
		path     string
		payload  any
		wantCode string
	}{
		{
			name:     "falta el campo matrix",
			path:     "/api/v1/qr",
			payload:  map[string]any{},
			wantCode: CodeInvalidBody,
		},
		{
			name:     "matriz sin filas",
			path:     "/api/v1/qr",
			payload:  map[string]any{"matrix": [][]float64{}},
			wantCode: "EMPTY_MATRIX",
		},
		{
			name:     "filas de distinto largo",
			path:     "/api/v1/qr",
			payload:  map[string]any{"matrix": [][]float64{{1, 2, 3}, {4, 5}}},
			wantCode: "RAGGED_ROWS",
		},
		{
			// Filas nulas: la primera fila no tiene columnas, de modo que la
			// matriz se descarta como vacía antes de mirar el resto.
			name:     "filas nulas",
			path:     "/api/v1/qr",
			payload:  map[string]any{"matrix": make([][]float64, 4)},
			wantCode: "EMPTY_MATRIX",
		},
		{
			name:     "modo inexistente",
			path:     "/api/v1/qr?mode=oblicuo",
			payload:  map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}},
			wantCode: CodeInvalidBody,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(t, app, http.MethodPost, tc.path, token, tc.payload)
			assertErrorCode(t, resp, http.StatusBadRequest, tc.wantCode)
		})
	}
}

func TestQRRejectsOversizedMatrix(t *testing.T) {
	stub := newStatsStub(t, nil, nil)
	app := NewApp(testConfig(stub.URL), discardLogger()) // MaxMatrixDimension = 8
	token := login(t, app)

	// 9×9 supera el límite configurado.
	oversized := make([][]float64, 9)
	for i := range oversized {
		oversized[i] = make([]float64, 9)
	}

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": oversized})

	assertErrorCode(t, resp, http.StatusBadRequest, "MATRIX_TOO_LARGE")
}

// TestQRRejectsUnrepresentableNumber documenta dónde se corta un valor que no
// cabe en un float64.
//
// JSON no tiene literales para NaN ni infinito, así que un valor no finito solo
// puede llegar como un número fuera de rango (1e400). El decodificador lo
// rechaza antes de que la matriz exista, por lo que el error es INVALID_BODY y
// no NON_FINITE_VALUE. Esa validación del paquete matrix sigue siendo útil como
// defensa en profundidad para quien use el paquete fuera de la capa HTTP, y se
// prueba en su propio test.
func TestQRRejectsUnrepresentableNumber(t *testing.T) {
	stub := newStatsStub(t, nil, nil)
	app := NewApp(testConfig(stub.URL), discardLogger())
	token := login(t, app)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qr",
		bytes.NewReader([]byte(`{"matrix": [[1, 2], [3, 1e400]]}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test devolvió error: %v", err)
	}
	assertErrorCode(t, resp, http.StatusBadRequest, CodeInvalidBody)
}

// --- Fallos del upstream ---------------------------------------------------

func TestUpstreamFailures(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantCode   string
	}{
		{
			name: "el upstream devuelve 500",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeUpstreamError,
		},
		{
			name: "el upstream rechaza el token",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeUpstreamError,
		},
		{
			name: "el upstream devuelve algo que no es JSON",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "<html>error del proxy</html>")
			},
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeUpstreamUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStatsStub(t, nil, tc.handler)
			app := NewApp(testConfig(stub.URL), discardLogger())
			token := login(t, app)

			resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
				map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

			assertErrorCode(t, resp, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestUpstreamUnreachable(t *testing.T) {
	// Puerto cerrado: la conexión se rechaza de inmediato.
	app := NewApp(testConfig("http://127.0.0.1:1"), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	assertErrorCode(t, resp, http.StatusBadGateway, CodeUpstreamUnavailable)
}

func TestUpstreamTimeout(t *testing.T) {
	stub := newStatsStub(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, fakeStatsResponse)
	})

	cfg := testConfig(stub.URL)
	cfg.StatsTimeout = 50 * time.Millisecond
	app := NewApp(cfg, discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	assertErrorCode(t, resp, http.StatusGatewayTimeout, CodeUpstreamTimeout)
}

// TestUpstreamRetryOnServerError comprueba que un 5xx transitorio se reintente
// y que el segundo intento pueda tener éxito.
func TestUpstreamRetryOnServerError(t *testing.T) {
	attempts := 0
	stub := newStatsStub(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fakeStatsResponse)
	})

	cfg := testConfig(stub.URL)
	cfg.StatsMaxRetries = 1
	app := NewApp(cfg, discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, se esperaba que el reintento tuviera éxito", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("intentos = %d, se esperaban 2 (original + 1 reintento)", attempts)
	}
}

// TestUpstreamDoesNotRetryClientError verifica que un 4xx no se reintente:
// repetir un request mal formado solo gastaría tiempo y carga.
func TestUpstreamDoesNotRetryClientError(t *testing.T) {
	attempts := 0
	stub := newStatsStub(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	})

	cfg := testConfig(stub.URL)
	cfg.StatsMaxRetries = 3
	app := NewApp(cfg, discardLogger())
	token := login(t, app)

	doRequest(t, app, http.MethodPost, "/api/v1/qr", token,
		map[string]any{"matrix": [][]float64{{1, 2}, {3, 4}}})

	if attempts != 1 {
		t.Errorf("intentos = %d, se esperaba 1 (los 4xx no se reintentan)", attempts)
	}
}

// --- Endpoint de rotación --------------------------------------------------

func TestRotate(t *testing.T) {
	app := NewApp(testConfig("http://unused"), discardLogger())
	token := login(t, app)

	resp := doRequest(t, app, http.MethodPost, "/api/v1/rotate", token,
		map[string]any{"matrix": [][]float64{{1, 2, 3}, {4, 5, 6}}})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", resp.StatusCode)
	}

	var body RotateResponse
	decodeBody(t, resp, &body)

	want := [][]float64{{4, 1}, {5, 2}, {6, 3}}
	if body.Rotated.Rows() != 3 || body.Rotated.Cols() != 2 {
		t.Fatalf("la rotada es %d×%d, se esperaba 3×2", body.Rotated.Rows(), body.Rotated.Cols())
	}
	for i := range want {
		for j := range want[i] {
			if body.Rotated[i][j] != want[i][j] {
				t.Errorf("rotada[%d][%d] = %g, se esperaba %g", i, j, body.Rotated[i][j], want[i][j])
			}
		}
	}
	if body.Meta.Degrees != 90 || body.Meta.Direction != "clockwise" {
		t.Errorf("meta = %+v, se esperaba rotación de 90° en sentido horario", body.Meta)
	}
}

// --- Salud y rutas ---------------------------------------------------------

func TestHealthIsPublic(t *testing.T) {
	app := NewApp(testConfig("http://127.0.0.1:1"), discardLogger())

	resp := doRequest(t, app, http.MethodGet, "/health", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200 sin token", resp.StatusCode)
	}

	var body HealthResponse
	decodeBody(t, resp, &body)
	if body.Status != "ok" {
		t.Errorf("status = %q, se esperaba \"ok\"", body.Status)
	}
	if body.Service != "qr-api-go" {
		t.Errorf("service = %q", body.Service)
	}
}

// TestReadinessReflectsUpstream comprueba que liveness y readiness respondan
// distinto: el servicio está vivo aunque su dependencia no lo esté.
func TestReadinessReflectsUpstream(t *testing.T) {
	t.Run("upstream disponible", func(t *testing.T) {
		stub := newStatsStub(t, nil, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		})
		app := NewApp(testConfig(stub.URL), discardLogger())

		resp := doRequest(t, app, http.MethodGet, "/health/ready", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, se esperaba 200", resp.StatusCode)
		}
	})

	t.Run("upstream caído", func(t *testing.T) {
		app := NewApp(testConfig("http://127.0.0.1:1"), discardLogger())

		resp := doRequest(t, app, http.MethodGet, "/health/ready", "", nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status = %d, se esperaba 503", resp.StatusCode)
		}

		var body HealthResponse
		decodeBody(t, resp, &body)
		if body.Upstream != "unreachable" {
			t.Errorf("upstream = %q, se esperaba \"unreachable\"", body.Upstream)
		}
	})
}

func TestUnknownRouteReturnsStructuredError(t *testing.T) {
	app := NewApp(testConfig("http://unused"), discardLogger())

	resp := doRequest(t, app, http.MethodGet, "/api/v1/inexistente", "", nil)

	assertErrorCode(t, resp, http.StatusNotFound, CodeNotFound)
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{"formato estándar", "Bearer abc.def.ghi", "abc.def.ghi", false},
		{"esquema en minúsculas", "bearer abc.def.ghi", "abc.def.ghi", false},
		{"esquema en mayúsculas", "BEARER abc.def.ghi", "abc.def.ghi", false},
		{"encabezado vacío", "", "", true},
		{"sin esquema", "abc.def.ghi", "", true},
		{"otro esquema", "Basic dXNlcjpwYXNz", "", true},
		{"token vacío", "Bearer ", "", true},
		{"solo espacios", "Bearer    ", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bearerToken(tc.header)

			if tc.wantErr {
				if err == nil {
					t.Errorf("se esperaba error para %q, se obtuvo %q", tc.header, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got != tc.want {
				t.Errorf("token = %q, se esperaba %q", got, tc.want)
			}
		})
	}
}
