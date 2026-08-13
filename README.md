# Coding Challenge Interseguro — Factorización QR y estadísticas

Dos APIs REST que se comunican por HTTP, más un frontend que las consume.

- **API Go (Fiber)** — recibe una matriz rectangular, calcula su **factorización QR** por reflexiones de Householder y envía las matrices resultantes a la segunda API.
- **API Node.js (Express + TypeScript)** — recibe esas matrices y calcula **máximo, mínimo, promedio, suma total** y si **alguna es diagonal**.
- **Frontend (React + Vite)** — pantalla de acceso, editor de matriz y visualización del resultado como la ecuación `A = Q · R`.

Todo va contenerizado, protegido con JWT y cubierto por pruebas unitarias y de integración.

---

## Arranque

Requisitos: Docker y Docker Compose. Nada más.

```bash
cp .env.example .env
docker compose up --build
```

| Servicio | URL | Notas |
| --- | --- | --- |
| Frontend | http://localhost:8081 | nginx sirve el build y hace de proxy a la API Go |
| API Go | http://localhost:8080 | Factorización QR y emisión de tokens |
| API Node | *(sin publicar)* | Solo alcanzable por la red interna de Docker |

Credenciales de demostración: usuario `demo`, contraseña `demo1234` (definidas en `.env`).

> La API Node no publica ningún puerto al host a propósito: solo la consume la API Go, así que exponerla ampliaría la superficie de ataque sin ningún beneficio.

---

## Arquitectura

```
   Navegador
       │
       ▼
   ┌─────────────────────────┐
   │  frontend  ·  nginx :80 │   sirve el build + proxy /api → api-go
   └───────────┬─────────────┘
               │  Authorization: Bearer <JWT>
               ▼
   ┌─────────────────────────┐
   │  api-go  ·  Fiber :8080 │   POST /api/v1/auth/login   emite el JWT
   │                         │   POST /api/v1/qr           Householder QR
   │                         │   POST /api/v1/rotate       rotación 90°
   └───────────┬─────────────┘
               │  HTTP · propaga Authorization y X-Request-ID
               ▼
   ┌─────────────────────────┐
   │ api-node · Express :3000│   POST /api/v1/statistics
   └─────────────────────────┘
```

**Flujo de una factorización:** el frontend pide el token → envía la matriz a la API Go → esta valida, factoriza y llama a la API Node con `{matrices: {q, r}}` → compone la respuesta final con `q`, `r`, `meta` y `statistics`.

El `X-Request-ID` viaja por toda la cadena, así que una sola búsqueda en los logs reconstruye la traza completa a través de ambos servicios.

---

## Prueba rápida por línea de comandos

Obtener un token:

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"demo","password":"demo1234"}'
```

Factorizar (reemplazar `$TOKEN` por el valor recibido):

```bash
curl -s -X POST http://localhost:8080/api/v1/qr -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"matrix":[[12,-51,4],[6,167,-68],[-4,24,-41]]}'
```

La respuesta trae `q`, `r`, `meta` y `statistics`. En `meta.residual` va el error relativo de reconstrucción `‖Q·R − A‖ / ‖A‖`: ronda `1e-16`, y es la forma de comprobar que el resultado es correcto sin confiar en el servicio.

El contrato completo, con todos los códigos de error, está en [docs/API.md](docs/API.md).

---

## Estructura

```
.
├── docker-compose.yml          orquestación de los tres servicios
├── .env.example                configuración compartida
├── docs/
│   ├── DECISIONS.md            decisiones de diseño y su justificación
│   ├── API.md                  contrato de ambas APIs
│   └── DEPLOY.md               despliegue en la nube
├── api-go/
│   ├── cmd/server/             punto de entrada
│   └── internal/
│       ├── matrix/             álgebra lineal: QR por Householder
│       ├── api/                rutas, handlers, middleware
│       ├── auth/               emisión y verificación de JWT
│       ├── client/             cliente HTTP hacia la API Node
│       └── config/             configuración desde el entorno
├── api-node/
│   ├── src/
│   │   ├── services/           cálculo de estadísticas (lógica pura)
│   │   ├── schemas/            validación con Zod
│   │   ├── routes/ middleware/ capa HTTP
│   └── tests/                  unitarias e integración
└── frontend/
    └── src/                    React + Vite + TypeScript
```

---

## Desarrollo sin Docker

Requiere Go 1.24+ y Node 22+.

API Node (puerto 3000):

```bash
cd api-node && npm install && JWT_SECRET=secreto-local npm run dev
```

API Go (puerto 8080):

```bash
cd api-go && JWT_SECRET=secreto-local DEMO_PASSWORD=demo1234 STATS_API_URL=http://localhost:3000 go run ./cmd/server
```

Frontend (puerto 5173, con proxy a la API Go):

```bash
cd frontend && npm install && npm run dev
```

`JWT_SECRET` debe ser idéntico en ambas APIs: la Go firma los tokens y la Node los verifica.

---

## Pruebas

```bash
cd api-go && go test ./... -cover
```

```bash
cd api-node && npm run test:coverage
```

| Paquete | Cobertura | Qué cubre |
| --- | --- | --- |
| `api-go/internal/matrix` | 97.2 % | QR verificada por propiedades: `Q·R = A`, `QᵀQ = I`, `R` triangular. Casos límite: rango deficiente, matriz de Hilbert, valores de 1e150 y 1e-150 |
| `api-go/internal/api` | 94.3 % | Rutas, autenticación, validación y los cinco modos de fallo del upstream |
| `api-go/internal/config` | 92.3 % | Valores por defecto y rechazo de configuraciones inválidas |
| `api-go/internal/auth` | 89.5 % | Emisión, expiración, firma incorrecta y el ataque `alg: none` |
| `api-node/src/services` | 100 % | Las cinco medidas, tolerancia relativa y suma compensada |
| `api-node` (total) | 92.1 % | 65 pruebas entre unitarias e integración |

Las pruebas de QR verifican **propiedades**, no valores precalculados: la factorización no es única —invertir el signo de una columna de `Q` y de la fila correspondiente de `R` da otra igualmente válida—, así que fijar valores esperados ataría el test a un detalle de implementación en vez de al contrato matemático.

---

## Decisiones destacadas

El detalle y la justificación de cada una está en [docs/DECISIONS.md](docs/DECISIONS.md).

1. **Se implementó QR, no rotación.** El enunciado se contradice: la sección de arquitectura habla de "rotación de la matriz" y la de funcionalidad pide "la factorización QR". Se siguió el requisito funcional explícito y se dejó además un endpoint `/api/v1/rotate` que cubre la lectura alternativa.
2. **Householder en vez de Gram-Schmidt.** Gram-Schmidt clásico pierde ortogonalidad en `Q` con matrices mal condicionadas; Householder es incondicionalmente estable. Hay un test con una matriz de Hilbert que lo demuestra.
3. **Tolerancia relativa por matriz para "es diagonal".** Comparar con `== 0` haría que ninguna matriz real pareciera diagonal, porque QR deja residuos de redondeo. La tolerancia se deriva de la magnitud de **cada** matriz: una global tomada de la mayor enmascararía valores significativos de las pequeñas.
4. **Suma compensada de Neumaier.** Sumar miles de valores de magnitudes distintas pierde precisión; el algoritmo conserva el error de cada paso y lo reintegra.
5. **El JWT del usuario se propaga entre servicios.** La API Node valida el mismo token que emitió la Go, en vez de una credencial de máquina: así la identidad del usuario final sobrevive el salto y la traza no se pierde.

---

## Despliegue en la nube

Las imágenes están listas para cualquier plataforma que ejecute contenedores. Los pasos concretos para Google Cloud Run, Render y Fly.io están en [docs/DEPLOY.md](docs/DEPLOY.md).

| Imagen | Tamaño |
| --- | --- |
| `api-go` | 28,2 MB |
| `frontend` | 92,9 MB |
| `api-node` | 254 MB |

Los tres contenedores corren sin privilegios de root, declaran `HEALTHCHECK` y hacen apagado ordenado ante `SIGTERM`.
