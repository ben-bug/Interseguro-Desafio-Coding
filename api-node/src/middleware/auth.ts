/**
 * Verificación del JWT emitido por la API Go.
 *
 * Este servicio no emite tokens: valida los que la API Go firmó con el mismo
 * secreto compartido y propagó en el encabezado Authorization. De ese modo la
 * identidad del usuario final sobrevive el salto entre servicios, en lugar de
 * reemplazarse por una credencial de máquina que perdería la trazabilidad.
 */

import type { NextFunction, Request, Response } from 'express';
import jwt from 'jsonwebtoken';
import { ApiError, ErrorCode } from '../errors.js';
import type { Config } from '../config.js';

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      /** Sujeto autenticado, disponible una vez que pasó el middleware. */
      authSubject?: string;
    }
  }
}

/** Middleware que exige un token Bearer válido. */
export function requireJwt(config: Config) {
  return (req: Request, _res: Response, next: NextFunction): void => {
    let token: string;
    try {
      token = extractBearerToken(req.header('authorization'));
    } catch (error) {
      next(error);
      return;
    }

    try {
      const payload = jwt.verify(token, config.jwtSecret, {
        // Restringir el algoritmo es imprescindible: sin esta lista, un token
        // con `alg: none` o firmado con otro esquema podría llegar a aceptarse.
        algorithms: ['HS256'],
        issuer: config.jwtIssuer,
        audience: config.jwtAudience,
      });

      const subject = typeof payload === 'string' ? undefined : payload.sub;
      if (!subject) {
        next(ApiError.unauthorized('el token no identifica a ningún sujeto'));
        return;
      }

      req.authSubject = subject;
      next();
    } catch (error) {
      if (error instanceof jwt.TokenExpiredError) {
        next(
          ApiError.unauthorized(
            'el token expiró: solicitar uno nuevo en POST /api/v1/auth/login de la API Go',
            ErrorCode.TOKEN_EXPIRED,
          ),
        );
        return;
      }
      // El motivo exacto no se expone: detallar por qué una firma no valida le
      // daría a un atacante información útil para afinar el siguiente intento.
      next(ApiError.unauthorized('el token es inválido'));
    }
  };
}

/**
 * Extrae el token del encabezado `Authorization: Bearer <token>`.
 * El esquema se compara sin distinguir mayúsculas, como exige RFC 7235.
 */
export function extractBearerToken(header: string | undefined): string {
  if (!header) {
    throw ApiError.unauthorized('falta el encabezado Authorization');
  }

  const separatorIndex = header.indexOf(' ');
  if (separatorIndex === -1) {
    throw ApiError.unauthorized("el encabezado Authorization debe tener el formato 'Bearer <token>'");
  }

  const scheme = header.slice(0, separatorIndex);
  const token = header.slice(separatorIndex + 1).trim();

  if (scheme.toLowerCase() !== 'bearer') {
    throw ApiError.unauthorized("el encabezado Authorization debe tener el formato 'Bearer <token>'");
  }
  if (!token) {
    throw ApiError.unauthorized('el token está vacío');
  }
  return token;
}
