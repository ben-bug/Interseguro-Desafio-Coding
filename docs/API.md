# Contrato de las APIs

Dos servicios. La **API Go** es la que consume el cliente; la **API Node** solo la invoca la API Go a través de la red interna.

Todas las respuestas son `application/json`. Todo error, en cualquiera de las dos APIs, tiene la misma forma:

```json
{
  "error": {
    "code": "RAGGED_ROWS",
    "message": "todas las filas deben tener el mismo largo: la fila 0 tiene 3 columnas y la fila 1 tiene 2",
    "details": { "expectedCols": 3, "rowIndex": 1, "actualCols": 2 },
    "requestId": "EU8pDNKE2HMGHYjLYjE30fwrwB9XZFdpNBDM_4UL7dk"
  }
}
```

`code` es estable y forma parte del contrato: conviene ramificar sobre él. `message` está escrito para personas y puede cambiar.

---

# API Go — factorización QR

Base: `http://localhost:8080`

## `POST /api/v1/auth/login`

Emite un JWT. Es el único endpoint que no requiere autenticación además de los de salud.

**Petición**

```json
{ "username": "demo", "password": "demo1234" }
```

**Respuesta `200`**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "tokenType": "Bearer",
  "expiresAt": "2026-08-13T12:30:54.338-04:00",
  "expiresIn": 900
}
```

`expiresAt` y `expiresIn` permiten renovar el token antes de que caduque, en lugar de esperar el primer 401.

**Errores**

| Código HTTP | `code` | Cuándo |
| --- | --- | --- |
| 401 | `INVALID_CREDENTIALS` | Usuario o contraseña incorrectos. El mensaje no distingue entre ambos, para no permitir enumerar usuarios. |
| 400 | `INVALID_BODY` | El cuerpo no es JSON válido. |

---

## `POST /api/v1/qr`

Factoriza la matriz y adjunta las estadísticas calculadas por la API Node.

**Autenticación:** `Authorization: Bearer <token>`

**Parámetros de consulta**

| Parámetro | Valores | Por defecto | Efecto |
| --- | --- | --- | --- |
| `mode` | `full`, `reduced` | `full` | `full` devuelve `Q` de m×m y `R` de m×n. `reduced`, cuando m > n, devuelve `Q` de m×n y `R` de n×n. |
| `withStats` | `false` | *(activo)* | Con `false` omite la llamada a la API Node. Útil para aislar en qué servicio está un fallo. |

**Petición**

```json
{ "matrix": [[12, -51, 4], [6, 167, -68], [-4, 24, -41]] }
```

**Respuesta `200`** (abreviada)

```json
{
  "q": [[-0.857142857142857, 0.394285714285714, 0.331428571428571], "…"],
  "r": [[-14, -21.000000000000007, 14.000000000000004], [0, -175.00000000000003, 69.99999999999999], [0, 0, -35]],
  "meta": {
    "rows": 3,
    "cols": 3,
    "mode": "full",
    "algorithm": "householder",
    "residual": 1.6028585704672133e-16,
    "durationMs": 0.04,
    "requestId": "xD-InoVPKqqyIkR_AwRiPY1_Q_gVGWBRyyRpiWZhJ2U"
  },
  "statistics": { "…": "respuesta de la API Node, ver más abajo" }
}
```

`meta.residual` es el error relativo de reconstrucción `‖Q·R − A‖_F / ‖A‖_F`. En doble precisión ronda 1e-16; sirve para comprobar el resultado sin confiar en el servicio.

Los elementos bajo la diagonal de `R` son ceros exactos, no residuos de redondeo: ver la decisión 4 en [DECISIONS.md](DECISIONS.md).

**Errores**

| Código HTTP | `code` | Cuándo |
| --- | --- | --- |
| 400 | `INVALID_BODY` | Falta el campo `matrix`, el JSON es inválido, `mode` no es `full` ni `reduced`, o un número no cabe en un `float64` (por ejemplo `1e400`). |
| 400 | `EMPTY_MATRIX` | La matriz no tiene filas, o su primera fila no tiene columnas. |
| 400 | `RAGGED_ROWS` | Las filas no tienen todas el mismo largo. `details` indica cuál rompe el rectángulo. |
| 400 | `NON_FINITE_VALUE` | La matriz contiene `NaN` o infinito. |
| 400 | `MATRIX_TOO_LARGE` | Supera `MAX_MATRIX_DIMENSION` (256 por defecto) en filas o columnas. |
| 401 | `UNAUTHORIZED` | Falta el token, el esquema no es `Bearer`, o la firma no valida. |
| 401 | `TOKEN_EXPIRED` | El token es válido pero venció. Se distingue porque el cliente puede resolverlo renovando la sesión. |
| 502 | `UPSTREAM_UNAVAILABLE` | No se pudo contactar a la API Node. |
| 502 | `UPSTREAM_ERROR` | La API Node respondió con un error. `details.upstreamStatus` trae su código. |
| 504 | `UPSTREAM_TIMEOUT` | La API Node no respondió dentro del plazo configurado. |

> `NaN` e infinito no tienen representación en JSON, así que en la práctica un valor no finito llega como un número fuera de rango y se rechaza antes, con `INVALID_BODY`. La validación `NON_FINITE_VALUE` existe como defensa en profundidad para quien use el paquete `matrix` fuera de la capa HTTP.

**Reintentos.** Ante un 5xx o un fallo de red, la API Go reintenta una vez con un backoff de 200 ms (configurable con `STATS_API_MAX_RETRIES`). Un 4xx no se reintenta: repetir una petición mal formada solo gastaría tiempo.

---

## `POST /api/v1/rotate`

Rota la matriz 90° en sentido horario. Existe para cubrir la lectura alternativa del enunciado; ver la decisión 1 en [DECISIONS.md](DECISIONS.md).

**Autenticación:** `Authorization: Bearer <token>`

**Petición**

```json
{ "matrix": [[1, 2, 3], [4, 5, 6]] }
```

**Respuesta `200`**

```json
{
  "rotated": [[4, 1], [5, 2], [6, 3]],
  "meta": { "rows": 3, "cols": 2, "direction": "clockwise", "degrees": 90, "requestId": "…" }
}
```

Los errores de validación son los mismos que en `/api/v1/qr`.

---

## `GET /health` y `GET /health/ready`

Públicos, sin autenticación.

`/health` es el chequeo de vitalidad y no consulta dependencias: responde `200` mientras el proceso esté sano.

```json
{ "status": "ok", "service": "qr-api-go", "version": "1.0.0" }
```

`/health/ready` es el chequeo de disponibilidad e incluye a la API Node. Responde `503` con `"status": "degraded"` y `"upstream": "unreachable"` cuando el upstream no responde.

---

# API Node — estadísticas

Base interna: `http://api-node:3000`. No se publica al host.

## `POST /api/v1/statistics`

**Autenticación:** `Authorization: Bearer <token>` — el mismo token que emitió la API Go, propagado por ella.

**Petición**

```json
{
  "matrices": {
    "q": [[1, 0], [0, 1]],
    "r": [[10, 20], [0, 40]]
  }
}
```

Las claves del objeto `matrices` son libres; la API Go envía `q` y `r`.

**Respuesta `200`**

```json
{
  "overall": { "max": 40, "min": 0, "average": 9, "sum": 72, "count": 8 },
  "perMatrix": {
    "q": {
      "max": 1, "min": 0, "average": 0.5, "sum": 2, "count": 4,
      "rows": 2, "cols": 2, "isSquare": true, "isDiagonal": true, "tolerance": 1e-9
    },
    "r": {
      "max": 40, "min": 0, "average": 17.5, "sum": 70, "count": 4,
      "rows": 2, "cols": 2, "isSquare": true, "isDiagonal": false, "tolerance": 4e-8
    }
  },
  "anyDiagonal": true,
  "toleranceFactor": 1e-9
}
```

`overall` responde lo que pide el enunciado: máximo, mínimo, promedio y suma total sobre **todas** las matrices. `perMatrix` es el desglose, que responde la pregunta inmediata siguiente —de cuál de las dos viene cada extremo— y no cuesta nada porque el recorrido ya está hecho.

`tolerance` es el umbral con que se evaluó `isDiagonal` en esa matriz, derivado de su propia magnitud: `max(1e-12, toleranceFactor · max|aᵢⱼ|)`. Difiere entre `Q` y `R` a propósito; ver la decisión 5 en [DECISIONS.md](DECISIONS.md).

**Errores**

| Código HTTP | `code` | Cuándo |
| --- | --- | --- |
| 400 | `INVALID_BODY` | El cuerpo no es JSON válido, falta `matrices`, o un elemento no es un número. |
| 400 | `NO_MATRICES` | `matrices` está vacío. |
| 400 | `EMPTY_MATRIX` | Una matriz no tiene filas, o su primera fila no tiene columnas. |
| 400 | `RAGGED_ROWS` | Las filas de una matriz no tienen todas el mismo largo. |
| 400 | `NON_FINITE_VALUE` | Una matriz contiene `NaN` o infinito. |
| 400 | `MATRIX_TOO_LARGE` | Una matriz supera `MAX_MATRIX_DIMENSION`. |
| 400 | `TOO_MANY_MATRICES` | Más de `MAX_MATRICES` (16 por defecto). |
| 401 | `UNAUTHORIZED` / `TOKEN_EXPIRED` | Igual que en la API Go. |
| 413 | `PAYLOAD_TOO_LARGE` | El cuerpo supera 16 MB. |
| 404 | `NOT_FOUND` | Ruta inexistente. |

## `GET /health`

Público. `{ "status": "ok", "service": "statistics-api-node", "version": "1.0.0" }`

---

# Trazabilidad

Ambas APIs aceptan y devuelven `X-Request-ID`. La API Go genera uno si el cliente no lo envía y lo propaga a la API Node, que lo adopta tras sanearlo. El identificador aparece en `meta.requestId`, en `error.requestId` y en cada línea de log de ambos servicios, de modo que una sola búsqueda reconstruye la traza completa.
