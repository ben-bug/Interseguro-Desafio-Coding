// Command server levanta la API Go de factorización QR.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/socius/interseguro-challenge/api-go/internal/api"
	"github.com/socius/interseguro-challenge/api-go/internal/config"
)

func main() {
	// Log estructurado en JSON: es lo que esperan los agregadores de las
	// plataformas cloud (Cloud Logging, CloudWatch) para indexar los campos.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuración inválida", slog.Any("error", err))
		os.Exit(1)
	}

	app := api.NewApp(cfg, logger)

	// Se escucha en 0.0.0.0 para ser alcanzable desde fuera del contenedor.
	addr := "0.0.0.0:" + cfg.Port

	// El servidor corre en su propia goroutine para que el hilo principal
	// quede libre esperando la señal de apagado.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("servidor iniciado",
			slog.String("addr", addr),
			slog.String("version", api.Version),
			slog.String("statsApi", cfg.StatsAPIURL),
		)
		serverErr <- app.Listen(addr, fiber.ListenConfig{
			// El banner ASCII de arranque ensuciaría el log estructurado: la
			// misma información ya sale como evento JSON justo arriba.
			DisableStartupMessage: true,
		})
	}()

	// SIGTERM es la señal que envía Docker (y Kubernetes) al detener un
	// contenedor; SIGINT llega con Ctrl+C en desarrollo.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("el servidor se detuvo con error", slog.Any("error", err))
			os.Exit(1)
		}

	case sig := <-shutdown:
		logger.Info("apagado solicitado, drenando conexiones", slog.String("signal", sig.String()))

		// Apagado ordenado: se da margen a los requests en curso para terminar
		// antes de cerrar. Sin esto, un despliegue cortaría respuestas a medio
		// camino y el cliente vería errores de red sin causa aparente.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := app.ShutdownWithContext(ctx); err != nil {
			logger.Error("el apagado ordenado no completó", slog.Any("error", err))
			os.Exit(1)
		}
		logger.Info("servidor detenido")
	}
}
