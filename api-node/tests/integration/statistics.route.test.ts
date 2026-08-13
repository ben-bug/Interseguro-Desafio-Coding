import { describe, expect, it } from 'vitest';
import request from 'supertest';
import jwt from 'jsonwebtoken';
import { pino } from 'pino';
import { createApp } from '../../src/app.js';
import type { Config } from '../../src/config.js';

const config: Config = {
  port: 0,
  jwtSecret: 'secreto-de-prueba',
  jwtIssuer: 'test-issuer',
  jwtAudience: 'test-audience',
  logLevel: 'silent',
  // Límites pequeños para poder ejercitar los rechazos sin construir matrices
  // enormes en el test.
  maxMatrixDimension: 4,
  maxMatrices: 3,
};

// El logger se silencia para que la salida del test no se llene de líneas de
// request.
const app = createApp(config, pino({ level: 'silent' }));

/** Firma un token con los mismos claims que emite la API Go. */
function signToken(overrides: Partial<jwt.SignOptions> = {}, secret = config.jwtSecret): string {
  return jwt.sign({}, secret, {
    subject: 'demo',
    issuer: config.jwtIssuer,
    audience: config.jwtAudience,
    algorithm: 'HS256',
    expiresIn: '15m',
    ...overrides,
  });
}

const authHeader = () => `Bearer ${signToken()}`;

const validBody = {
  matrices: {
    q: [
      [1, 0],
      [0, 1],
    ],
    r: [
      [10, 20],
      [0, 40],
    ],
  },
};

describe('POST /api/v1/statistics', () => {
  it('calcula las estadísticas de las matrices recibidas', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .send(validBody);

    expect(response.status).toBe(200);
    expect(response.body.overall).toMatchObject({
      max: 40,
      min: 0,
      sum: 72,
      count: 8,
      average: 9,
    });
    expect(response.body.anyDiagonal).toBe(true);
    expect(Object.keys(response.body.perMatrix)).toEqual(['q', 'r']);
    expect(response.body.perMatrix.q.isDiagonal).toBe(true);
    expect(response.body.perMatrix.r.isDiagonal).toBe(false);
  });

  it('devuelve el identificador de correlación que envió la API Go', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .set('X-Request-ID', 'traza-123')
      .send(validBody);

    expect(response.headers['x-request-id']).toBe('traza-123');
  });

  it('genera un identificador propio si el request llega sin él', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .send(validBody);

    expect(response.headers['x-request-id']).toBeTruthy();
  });

  it('descarta un identificador con caracteres no permitidos', async () => {
    // El valor se copia a un encabezado de respuesta y a los logs, así que no
    // puede aceptarse tal cual venga.
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .set('X-Request-ID', 'traza con espacios y <script>')
      .send(validBody);

    expect(response.headers['x-request-id']).not.toBe('traza con espacios y <script>');
  });
});

describe('autenticación', () => {
  it.each([
    ['sin encabezado', undefined],
    ['esquema incorrecto', 'Basic dXNlcjpwYXNz'],
    ['token vacío', 'Bearer '],
    ['token inventado', 'Bearer no-es-un-jwt'],
  ])('rechaza el acceso %s', async (_name, header) => {
    const req = request(app).post('/api/v1/statistics');
    if (header) req.set('Authorization', header);

    const response = await req.send(validBody);

    expect(response.status).toBe(401);
    expect(response.body.error.code).toBe('UNAUTHORIZED');
  });

  it('rechaza un token firmado con otro secreto', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', `Bearer ${signToken({}, 'otro-secreto')}`)
      .send(validBody);

    expect(response.status).toBe(401);
    expect(response.body.error.code).toBe('UNAUTHORIZED');
  });

  it('distingue un token expirado para que el cliente sepa que debe renovarlo', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', `Bearer ${signToken({ expiresIn: '-1m' })}`)
      .send(validBody);

    expect(response.status).toBe(401);
    expect(response.body.error.code).toBe('TOKEN_EXPIRED');
  });

  it.each([
    ['emisor distinto', { issuer: 'emisor-falso' }],
    ['audiencia distinta', { audience: 'audiencia-falsa' }],
  ])('rechaza un token con %s', async (_name, overrides) => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', `Bearer ${signToken(overrides)}`)
      .send(validBody);

    expect(response.status).toBe(401);
  });

  /**
   * El ataque clásico contra JWT: presentar un token con `alg: none` y sin
   * firma. La lista explícita de algoritmos en la verificación debe bloquearlo.
   */
  it('rechaza un token con alg=none', async () => {
    const header = Buffer.from(JSON.stringify({ alg: 'none', typ: 'JWT' })).toString('base64url');
    const payload = Buffer.from(
      JSON.stringify({
        sub: 'atacante',
        iss: config.jwtIssuer,
        aud: config.jwtAudience,
        exp: Math.floor(Date.now() / 1000) + 3600,
      }),
    ).toString('base64url');

    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', `Bearer ${header}.${payload}.`)
      .send(validBody);

    expect(response.status).toBe(401);
  });
});

describe('validación de la entrada', () => {
  const cases: Array<[string, unknown, string]> = [
    ['cuerpo vacío', {}, 'INVALID_BODY'],
    ['matrices no es un objeto', { matrices: 'q' }, 'INVALID_BODY'],
    ['sin ninguna matriz', { matrices: {} }, 'NO_MATRICES'],
    ['matriz sin filas', { matrices: { q: [] } }, 'EMPTY_MATRIX'],
    ['matriz con una fila vacía', { matrices: { q: [[]] } }, 'EMPTY_MATRIX'],
    [
      'filas de distinto largo',
      { matrices: { q: [[1, 2, 3], [4, 5]] } },
      'RAGGED_ROWS',
    ],
    [
      'elemento que no es número',
      { matrices: { q: [[1, 'dos']] } },
      'INVALID_BODY',
    ],
    [
      'matriz que supera la dimensión máxima',
      { matrices: { q: [[1, 2, 3, 4, 5]] } },
      'MATRIX_TOO_LARGE',
    ],
    [
      'demasiadas matrices',
      { matrices: { a: [[1]], b: [[2]], c: [[3]], d: [[4]] } },
      'TOO_MANY_MATRICES',
    ],
  ];

  it.each(cases)('rechaza %s con el código %s', async (_name, body, expectedCode) => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .send(body as object);

    expect(response.status).toBe(400);
    expect(response.body.error.code).toBe(expectedCode);
    expect(response.body.error.message).toBeTruthy();
    expect(response.body.error.requestId).toBeTruthy();
  });

  it('detalla qué fila rompe el rectángulo', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .send({ matrices: { q: [[1, 2, 3], [4, 5, 6], [7, 8]] } });

    expect(response.body.error.details).toMatchObject({
      rowIndex: 2,
      expectedCols: 3,
      actualCols: 2,
    });
  });

  it('rechaza un JSON malformado como error del cliente, no del servidor', async () => {
    const response = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', authHeader())
      .set('Content-Type', 'application/json')
      .send('{"matrices": {');

    expect(response.status).toBe(400);
    expect(response.body.error.code).toBe('INVALID_BODY');
  });
});

describe('rutas y salud', () => {
  it('GET /health responde sin token', async () => {
    const response = await request(app).get('/health');

    expect(response.status).toBe(200);
    expect(response.body).toMatchObject({ status: 'ok', service: 'statistics-api-node' });
  });

  it('devuelve un 404 con el formato de error de la API', async () => {
    const response = await request(app).get('/api/v1/inexistente');

    expect(response.status).toBe(404);
    expect(response.body.error.code).toBe('NOT_FOUND');
    expect(response.body.error.requestId).toBeTruthy();
  });

  it('no expone la cabecera X-Powered-By', async () => {
    const response = await request(app).get('/health');

    expect(response.headers['x-powered-by']).toBeUndefined();
  });
});
