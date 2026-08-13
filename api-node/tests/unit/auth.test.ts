import { describe, expect, it } from 'vitest';
import { extractBearerToken } from '../../src/middleware/auth.js';
import { ApiError } from '../../src/errors.js';

describe('extractBearerToken', () => {
  it.each([
    ['formato estándar', 'Bearer abc.def.ghi'],
    ['esquema en minúsculas', 'bearer abc.def.ghi'],
    ['esquema en mayúsculas', 'BEARER abc.def.ghi'],
    ['con espacios sobrantes', 'Bearer   abc.def.ghi  '],
  ])('extrae el token con %s', (_name, header) => {
    expect(extractBearerToken(header)).toBe('abc.def.ghi');
  });

  it.each([
    ['encabezado ausente', undefined],
    ['encabezado vacío', ''],
    ['sin esquema', 'abc.def.ghi'],
    ['otro esquema', 'Basic dXNlcjpwYXNz'],
    ['token vacío', 'Bearer '],
    ['solo espacios', 'Bearer    '],
  ])('rechaza %s', (_name, header) => {
    expect(() => extractBearerToken(header)).toThrow(ApiError);
  });

  it('responde con status 401', () => {
    try {
      extractBearerToken(undefined);
      expect.unreachable('debía lanzar');
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).status).toBe(401);
    }
  });
});
