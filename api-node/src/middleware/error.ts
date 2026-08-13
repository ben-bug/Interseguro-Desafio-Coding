/**
 * Manejo centralizado de errores.
 *
 * Todo error termina acá y sale con la misma forma. Centralizarlo garantiza que
 * ninguna ruta invente su propio formato y que los detalles internos no se
 * filtren al cliente por descuido.
 */

import type { NextFunction, Request, Response } from 'express';
import type { Logger } from 'pino';
import { ApiError, ErrorCode, type ErrorResponse } from '../errors.js';

/** Responde 404 a cualquier ruta no registrada. */
export function notFoundHandler() {
  return (req: Request, _res: Response, next: NextFunction): void => {
    next(ApiError.notFound(`la ruta ${req.method} ${req.path} no existe`));
  };
}

/**
 * Manejador final de errores.
 *
 * Express identifica un manejador de errores por su aridad de cuatro
 * parámetros, de modo que `next` debe declararse aunque no se use.
 */
export function errorHandler(logger: Logger) {
  return (rawError: unknown, req: Request, res: Response, _next: NextFunction): void => {
    const requestId = req.requestId;
    const error = normalizeBodyParserError(rawError);

    if (error instanceof ApiError) {
      // Los errores esperados (validación, autenticación) no son incidentes:
      // se registran en warn para no contaminar las alertas de error.
      logger.warn(
        { requestId, code: error.code, status: error.status, path: req.path },
        error.message,
      );

      const body: ErrorResponse = {
        error: {
          code: error.code,
          message: error.message,
          ...(error.details ? { details: error.details } : {}),
          requestId,
        },
      };
      res.status(error.status).json(body);
      return;
    }

    // Un error no contemplado sí es un incidente: se registra completo, con
    // stack, pero al cliente solo le llega un mensaje genérico. Devolver el
    // detalle interno filtraría rutas de archivos y estructura del código.
    logger.error({ requestId, path: req.path, err: error }, 'error no controlado');

    const body: ErrorResponse = {
      error: {
        code: ErrorCode.INTERNAL_ERROR,
        message: 'error interno del servidor',
        requestId,
      },
    };
    res.status(500).json(body);
  };
}

/** Forma de los errores que emite body-parser, el parser que usa express.json(). */
interface BodyParserError extends Error {
  type?: string;
  status?: number;
}

/**
 * Convierte los errores de `express.json()` en ApiError.
 *
 * Sin esta traducción, un cuerpo con JSON malformado terminaría en la rama del
 * error no controlado y saldría como 500: el cliente no sabría que el problema
 * es suyo y corregible, y en el servidor cada request mal formado se registraría
 * como incidente, disparando alertas sin motivo.
 */
function normalizeBodyParserError(error: unknown): unknown {
  if (!(error instanceof Error)) return error;

  const { type, status } = error as BodyParserError;

  if (type === 'entity.parse.failed') {
    return ApiError.badRequest(ErrorCode.INVALID_BODY, 'el cuerpo del request no es JSON válido');
  }
  if (type === 'entity.too.large') {
    return new ApiError(
      413,
      ErrorCode.PAYLOAD_TOO_LARGE,
      'el cuerpo del request supera el tamaño máximo permitido',
    );
  }
  // Resto de fallos de parseo (charset o codificación no soportados): siguen
  // siendo problemas del request, no del servidor.
  if (typeof type === 'string' && type.startsWith('entity.') && status && status < 500) {
    return new ApiError(status, ErrorCode.INVALID_BODY, error.message);
  }

  return error;
}

/**
 * Registra una línea por request al terminar la respuesta.
 *
 * Usa las mismas claves que el logger de la API Go (`requestId`, `method`,
 * `path`, `status`, `durationMs`), de modo que una sola consulta sirve para
 * seguir una traza a través de ambos servicios.
 */
export function requestLogger(logger: Logger) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const start = process.hrtime.bigint();

    // 'finish' se dispara cuando la respuesta se envió por completo, que es el
    // único momento en que el status y la duración son definitivos.
    res.on('finish', () => {
      const durationMs = Number(process.hrtime.bigint() - start) / 1e6;
      logger.info(
        {
          requestId: req.requestId,
          method: req.method,
          path: req.path,
          status: res.statusCode,
          durationMs: Number(durationMs.toFixed(3)),
        },
        'request finalizado',
      );
    });

    next();
  };
}
