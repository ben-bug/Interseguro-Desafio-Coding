/**
 * Cliente de la API Go.
 *
 * Todas las rutas son relativas: en desarrollo las redirige el proxy de Vite y
 * en producción las redirige nginx. Así el frontend no necesita conocer la URL
 * de la API ni cambiarla entre entornos, y no hay CORS que resolver.
 */

export type Matrix = number[][];

export interface LoginResponse {
  token: string;
  tokenType: string;
  expiresAt: string;
  expiresIn: number;
}

export interface MatrixStats {
  max: number;
  min: number;
  average: number;
  sum: number;
  count: number;
  rows: number;
  cols: number;
  isSquare: boolean;
  isDiagonal: boolean;
  tolerance: number;
}

export interface Statistics {
  overall: {
    max: number;
    min: number;
    average: number;
    sum: number;
    count: number;
  };
  perMatrix: Record<string, MatrixStats>;
  anyDiagonal: boolean;
  toleranceFactor: number;
}

export interface QRResult {
  q: Matrix;
  r: Matrix;
  meta: {
    rows: number;
    cols: number;
    mode: string;
    algorithm: string;
    residual: number;
    durationMs: number;
    requestId?: string;
  };
  statistics?: Statistics;
}

/** Error de la API, con el código estable que devuelven ambos servicios. */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string; details?: Record<string, unknown> };
}

/** Envía la petición y convierte cualquier fallo en un ApiError legible. */
async function send<T>(path: string, init: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, init);
  } catch {
    // fetch solo rechaza por fallos de red, no por códigos de error HTTP.
    throw new ApiError('NETWORK_ERROR', 'No se pudo contactar al servidor. ¿Está la API en marcha?');
  }

  const body: unknown = await response.json().catch(() => null);

  if (!response.ok) {
    const payload = (body as ErrorBody)?.error;
    throw new ApiError(
      payload?.code ?? 'UNKNOWN_ERROR',
      payload?.message ?? `El servidor respondió ${response.status}.`,
      payload?.details,
    );
  }

  return body as T;
}

export function login(username: string, password: string): Promise<LoginResponse> {
  return send<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
}

export function factorize(
  token: string,
  matrix: Matrix,
  mode: 'full' | 'reduced',
): Promise<QRResult> {
  return send<QRResult>(`/api/v1/qr?mode=${mode}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ matrix }),
  });
}
