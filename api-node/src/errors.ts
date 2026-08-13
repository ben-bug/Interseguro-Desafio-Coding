/**
 * Contrato de errores de la API.
 *
 * Replica deliberadamente el formato de la API Go: ambos servicios responden
 * `{ error: { code, message, details, requestId } }`, de modo que el frontend
 * tiene un único camino de manejo de errores sin importar cuál falló.
 */

/**
 * Códigos de error estables. Forman parte del contrato público: el cliente
 * puede ramificar sobre ellos sin parsear el mensaje, que está escrito para
 * personas y puede cambiar.
 */
export const ErrorCode = {
  INVALID_BODY: 'INVALID_BODY',
  NO_MATRICES: 'NO_MATRICES',
  EMPTY_MATRIX: 'EMPTY_MATRIX',
  RAGGED_ROWS: 'RAGGED_ROWS',
  NON_FINITE_VALUE: 'NON_FINITE_VALUE',
  MATRIX_TOO_LARGE: 'MATRIX_TOO_LARGE',
  TOO_MANY_MATRICES: 'TOO_MANY_MATRICES',
  PAYLOAD_TOO_LARGE: 'PAYLOAD_TOO_LARGE',
  UNAUTHORIZED: 'UNAUTHORIZED',
  TOKEN_EXPIRED: 'TOKEN_EXPIRED',
  NOT_FOUND: 'NOT_FOUND',
  INTERNAL_ERROR: 'INTERNAL_ERROR',
} as const;

export type ErrorCodeValue = (typeof ErrorCode)[keyof typeof ErrorCode];

/** Cuerpo de un error. */
export interface ErrorPayload {
  code: ErrorCodeValue;
  message: string;
  details?: Record<string, unknown>;
  requestId?: string;
}

/** Respuesta de error: el payload va bajo `error` para no confundirse con un éxito. */
export interface ErrorResponse {
  error: ErrorPayload;
}

/** Error que ya sabe con qué status HTTP debe responderse. */
export class ApiError extends Error {
  override readonly name = 'ApiError';

  constructor(
    readonly status: number,
    readonly code: ErrorCodeValue,
    message: string,
    readonly details?: Record<string, unknown>,
  ) {
    super(message);
  }

  /** 400: el request no cumple el contrato de entrada. */
  static badRequest(code: ErrorCodeValue, message: string, details?: Record<string, unknown>) {
    return new ApiError(400, code, message, details);
  }

  /** 401: falta el token, es inválido o expiró. */
  static unauthorized(message: string, code: ErrorCodeValue = ErrorCode.UNAUTHORIZED) {
    return new ApiError(401, code, message);
  }

  /** 404: ruta inexistente. */
  static notFound(message: string) {
    return new ApiError(404, ErrorCode.NOT_FOUND, message);
  }
}
