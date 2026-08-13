import { describe, expect, it } from 'vitest';
import { ConfigError, loadConfig } from '../../src/config.js';

/** Entorno mínimo con el que loadConfig debe tener éxito. */
const validEnv = { JWT_SECRET: 'secreto-de-prueba' } as NodeJS.ProcessEnv;

describe('loadConfig', () => {
  it('aplica los valores por defecto documentados', () => {
    const config = loadConfig(validEnv);

    expect(config).toMatchObject({
      port: 3000,
      jwtIssuer: 'interseguro-qr-api',
      jwtAudience: 'interseguro-clients',
      logLevel: 'info',
      maxMatrixDimension: 256,
      maxMatrices: 16,
    });
  });

  it('toma los valores del entorno cuando están presentes', () => {
    const config = loadConfig({
      ...validEnv,
      NODE_API_PORT: '4000',
      JWT_ISSUER: 'otro-emisor',
      JWT_AUDIENCE: 'otra-audiencia',
      LOG_LEVEL: 'debug',
      MAX_MATRIX_DIMENSION: '64',
      MAX_MATRICES: '4',
    });

    expect(config).toMatchObject({
      port: 4000,
      jwtIssuer: 'otro-emisor',
      jwtAudience: 'otra-audiencia',
      logLevel: 'debug',
      maxMatrixDimension: 64,
      maxMatrices: 4,
    });
  });

  it('no arranca sin JWT_SECRET', () => {
    expect(() => loadConfig({})).toThrow(ConfigError);
  });

  it.each([
    ['puerto fuera de rango', { NODE_API_PORT: '70000' }],
    ['dimensión máxima en cero', { MAX_MATRIX_DIMENSION: '0' }],
    ['cantidad máxima de matrices en cero', { MAX_MATRICES: '0' }],
  ])('rechaza una configuración inválida: %s', (_name, overrides) => {
    expect(() => loadConfig({ ...validEnv, ...overrides })).toThrow(ConfigError);
  });

  it('cae al valor por defecto ante un número mal escrito', () => {
    // Un error de tipeo es recuperable: no debe impedir el arranque por sí solo.
    const config = loadConfig({ ...validEnv, NODE_API_PORT: 'tres-mil' });

    expect(config.port).toBe(3000);
  });
});
