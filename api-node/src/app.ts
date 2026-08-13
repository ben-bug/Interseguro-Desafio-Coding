import express, { type Express } from 'express';
import { pino, type Logger } from 'pino';
import type { Config } from './config.js';
import { errorHandler, notFoundHandler, requestLogger } from './middleware/error.js';
import { requestId } from './middleware/requestId.js';
import { healthRouter } from './routes/health.route.js';
import { statisticsRouter } from './routes/statistics.route.js';

/**
 * Construye la aplicación Express sin abrir ningún puerto.
 *
 * Separar la construcción del arranque permite que los tests la ejerciten con
 * Supertest en memoria, sin sockets ni puertos ocupados.
 */
export function createApp(config: Config, logger: Logger = createLogger(config)): Express {
  const app = express();

  // Se confía en el primer proxy de la cadena para resolver la IP de origen:
  // en Docker Compose y en las plataformas cloud el tráfico siempre llega a
  // través de uno.
  app.set('trust proxy', 1);
  // La cabecera X-Powered-By solo informa a un atacante qué stack se ejecuta.
  app.disable('x-powered-by');

  // El orden importa: requestId primero, para que el logger y el manejador de
  // errores ya dispongan del identificador de correlación.
  app.use(requestId());
  app.use(requestLogger(logger));

  // El límite del cuerpo acota el gasto de memoria: una matriz de 256×256 en
  // JSON ocupa unos pocos MB, así que 16 MB deja margen de sobra y a la vez
  // impide que un cuerpo enorme se materialice antes de validarse.
  app.use(express.json({ limit: '16mb' }));

  app.use(healthRouter());
  app.use('/api/v1', statisticsRouter(config));

  // Ambos van al final: el 404 solo aplica si ninguna ruta coincidió, y el
  // manejador de errores debe ser el último middleware de la cadena.
  app.use(notFoundHandler());
  app.use(errorHandler(logger));

  return app;
}

/**
 * Crea el logger estructurado.
 *
 * Emite JSON en una línea por evento, que es el formato que los agregadores de
 * las plataformas cloud (Cloud Logging, CloudWatch) indexan sin configuración
 * adicional.
 */
export function createLogger(config: Config): Logger {
  return pino({
    level: config.logLevel,
    // El campo por defecto es `time` en milisegundos; ISO-8601 es legible tanto
    // para una persona como para el agregador.
    timestamp: () => `,"time":"${new Date().toISOString()}"`,
    base: { service: 'statistics-api-node' },
  });
}
