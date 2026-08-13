/** Punto de entrada: levanta la API de estadísticas. */

import { createApp, createLogger } from './app.js';
import { ConfigError, loadConfig } from './config.js';

function main(): void {
  let config;
  try {
    config = loadConfig();
  } catch (error) {
    if (error instanceof ConfigError) {
      // Todavía no hay logger configurado, así que el error va a stderr crudo.
      process.stderr.write(`configuración inválida: ${error.message}\n`);
      process.exit(1);
    }
    throw error;
  }

  const logger = createLogger(config);
  const app = createApp(config, logger);

  // 0.0.0.0 en lugar de localhost: dentro de un contenedor, escuchar solo en la
  // interfaz de loopback lo haría inalcanzable desde fuera.
  const server = app.listen(config.port, '0.0.0.0', () => {
    logger.info({ port: config.port }, 'servidor iniciado');
  });

  // Apagado ordenado: se deja de aceptar conexiones y se espera a que terminen
  // los requests en curso. Sin esto, un despliegue cortaría respuestas a medio
  // camino y el cliente vería errores de red sin causa aparente.
  const shutdown = (signal: string): void => {
    logger.info({ signal }, 'apagado solicitado, drenando conexiones');

    // Red de seguridad: si alguna conexión no cierra, no se puede quedar
    // colgado indefinidamente bloqueando el despliegue.
    const forceExit = setTimeout(() => {
      logger.error('el apagado ordenado no completó a tiempo, forzando la salida');
      process.exit(1);
    }, 10_000);
    forceExit.unref();

    server.close((error) => {
      if (error) {
        logger.error({ err: error }, 'error al cerrar el servidor');
        process.exit(1);
      }
      logger.info('servidor detenido');
      process.exit(0);
    });
  };

  // SIGTERM es la señal que envía Docker al detener un contenedor;
  // SIGINT llega con Ctrl+C en desarrollo.
  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));
}

main();
