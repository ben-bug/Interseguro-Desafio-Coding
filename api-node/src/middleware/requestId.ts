import { randomUUID } from 'node:crypto';
import type { NextFunction, Request, Response } from 'express';

/** Encabezado con que viaja el identificador de correlación entre servicios. */
export const REQUEST_ID_HEADER = 'x-request-id';

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      /** Identificador de correlación del request. */
      requestId: string;
    }
  }
}

/**
 * Adopta el identificador que envía la API Go, o genera uno si el request llega
 * directo.
 *
 * Conservar el mismo identificador a lo largo de la cadena es lo que permite
 * seguir una traza completa —frontend, API Go y API Node— con una sola búsqueda
 * en los logs.
 *
 * El valor recibido se sanea antes de usarse: se acepta solo un subconjunto
 * seguro de caracteres y se acota el largo, porque termina copiado en un
 * encabezado de respuesta y en las líneas de log.
 */
export function requestId() {
  return (req: Request, res: Response, next: NextFunction): void => {
    const incoming = req.header(REQUEST_ID_HEADER);
    req.requestId = isSafeRequestId(incoming) ? incoming : randomUUID();
    res.setHeader(REQUEST_ID_HEADER, req.requestId);
    next();
  };
}

function isSafeRequestId(value: string | undefined): value is string {
  return value !== undefined && value.length > 0 && value.length <= 128 && /^[\w.:-]+$/.test(value);
}
