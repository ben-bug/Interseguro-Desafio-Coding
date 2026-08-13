/**
 * Configuración del servicio, resuelta desde variables de entorno.
 *
 * Se valida al arrancar y el proceso no levanta si algo falta: es preferible
 * fallar de inmediato, cuando el problema es evidente, que descubrirlo en el
 * primer request en producción.
 */

export interface Config {
  /** Puerto TCP en que escucha el servidor. */
  port: number;
  /** Secreto HS256 compartido con la API Go, que es quien emite los tokens. */
  jwtSecret: string;
  /** Claims `iss` y `aud` que debe traer todo token aceptado. */
  jwtIssuer: string;
  jwtAudience: string;
  /** Nivel de log de pino. */
  logLevel: string;
  /** Límite de filas y columnas por matriz. */
  maxMatrixDimension: number;
  /** Límite de matrices por request. */
  maxMatrices: number;
}

/** Error de configuración: distingue un arranque mal configurado de un bug. */
export class ConfigError extends Error {
  override readonly name = 'ConfigError';
}

/**
 * Construye la configuración a partir del entorno.
 *
 * Recibe `env` como parámetro en lugar de leer `process.env` directamente para
 * poder probar cada combinación sin ensuciar el entorno del proceso de test.
 */
export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const config: Config = {
    port: readInt(env.NODE_API_PORT, 3000),
    jwtSecret: env.JWT_SECRET ?? '',
    jwtIssuer: env.JWT_ISSUER || 'interseguro-qr-api',
    jwtAudience: env.JWT_AUDIENCE || 'interseguro-clients',
    logLevel: env.LOG_LEVEL || 'info',
    maxMatrixDimension: readInt(env.MAX_MATRIX_DIMENSION, 256),
    maxMatrices: readInt(env.MAX_MATRICES, 16),
  };

  // Sin secreto no hay forma de verificar ninguna firma. Generar uno al vuelo
  // sería peor: los tokens emitidos por la API Go dejarían de validar y el
  // fallo aparecería recién en el primer request, no al arrancar.
  if (!config.jwtSecret) {
    throw new ConfigError(
      'JWT_SECRET es obligatorio y debe coincidir con el de la API Go (ver .env.example)',
    );
  }
  if (config.port < 1 || config.port > 65535) {
    throw new ConfigError(`NODE_API_PORT fuera de rango: ${config.port}`);
  }
  if (config.maxMatrixDimension < 1) {
    throw new ConfigError(`MAX_MATRIX_DIMENSION debe ser positivo: ${config.maxMatrixDimension}`);
  }
  if (config.maxMatrices < 1) {
    throw new ConfigError(`MAX_MATRICES debe ser positivo: ${config.maxMatrices}`);
  }

  return config;
}

/**
 * Devuelve el valor por defecto si la variable está ausente o no es un entero.
 * Un valor mal escrito no debe impedir el arranque por sí solo: las
 * validaciones de rango de arriba se encargan de los casos imposibles.
 */
function readInt(raw: string | undefined, fallback: number): number {
  if (raw === undefined || raw.trim() === '') return fallback;
  const parsed = Number.parseInt(raw, 10);
  return Number.isNaN(parsed) ? fallback : parsed;
}
