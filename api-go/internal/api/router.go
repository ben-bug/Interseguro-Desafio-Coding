package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/socius/interseguro-challenge/api-go/internal/auth"
	"github.com/socius/interseguro-challenge/api-go/internal/client"
	"github.com/socius/interseguro-challenge/api-go/internal/config"
)

// NewApp arma la aplicación Fiber con su cadena de middleware y sus rutas.
//
// Devolver *fiber.App en vez de arrancar el servidor acá permite que los tests
// usen app.Test() sin abrir un puerto real.
func NewApp(cfg config.Config, logger *slog.Logger) *fiber.App {
	statsClient := client.NewStatsClient(cfg.StatsAPIURL, cfg.StatsTimeout, cfg.StatsMaxRetries, logger)
	authManager := auth.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTTTL)
	handler := NewHandler(cfg, statsClient, authManager, logger)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
		AppName:      "Interseguro QR API (Go + Fiber)",
		// El cuerpo se acota a 16 MB: una matriz de 256×256 en JSON ocupa unos
		// pocos MB, así que este techo deja margen suficiente y a la vez impide
		// que un cuerpo enorme agote la memoria antes de llegar a validarse.
		BodyLimit: 16 * 1024 * 1024,
	})

	// El orden importa. recover va primero para atrapar los panics de todo lo
	// que venga después; requestid antes del logger para que el identificador
	// ya exista al momento de registrar la línea.
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(RequestLogger(logger))
	app.Use(cors.New(cors.Config{
		// El frontend se sirve desde otro origen. En un despliegue real esta
		// lista se restringe por entorno mediante configuración.
		AllowOrigins: []string{"*"},
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodOptions},
		AllowHeaders: []string{fiber.HeaderContentType, fiber.HeaderAuthorization, "X-Request-ID"},
	}))

	// Endpoints de salud: públicos, porque los consultan el orquestador y el
	// balanceador, que no tienen credenciales.
	app.Get("/health", handler.Health)
	app.Get("/health/ready", handler.Ready)

	v1 := app.Group("/api/v1")
	v1.Post("/auth/login", handler.Login)

	// El middleware de autenticación se declara ruta por ruta en lugar de sobre
	// un grupo. Un grupo con prefijo vacío se comporta como un Use() sobre todo
	// /api/v1: interceptaría también las rutas inexistentes —devolviendo 401
	// donde corresponde un 404— y dejaría que el carácter público o protegido
	// de cada endpoint dependiera del orden en que se registró.
	requireAuth := RequireJWT(authManager)
	v1.Post("/qr", requireAuth, handler.QR)
	v1.Post("/rotate", requireAuth, handler.Rotate)

	return app
}
