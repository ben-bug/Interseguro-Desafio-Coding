# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Etapa de compilación
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Las dependencias se copian y descargan antes que el código fuente: mientras
# go.mod y go.sum no cambien, Docker reutiliza esta capa y no vuelve a bajar
# los módulos en cada build.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev

# CGO_ENABLED=0 produce un binario estático, sin dependencias de libc, que puede
# ejecutarse sobre una imagen base mínima.
#   -trimpath  elimina las rutas absolutas de compilación del binario
#   -s -w      descartan la tabla de símbolos y la información de depuración
#   -X         inyecta la versión del build, visible en GET /health
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/socius/interseguro-challenge/api-go/internal/api.Version=${VERSION}" \
    -o /out/server ./cmd/server

# ---------------------------------------------------------------------------
# Etapa de ejecución
# ---------------------------------------------------------------------------
# Alpine en lugar de `scratch` o distroless: pesa unos pocos MB más, pero trae
# el wget de busybox que necesita el HEALTHCHECK y permite entrar a diagnosticar
# un contenedor en problemas. También aporta los certificados raíz, necesarios
# si la API de estadísticas se expone por HTTPS.
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

# Usuario sin privilegios: si el proceso se ve comprometido, no corre como root.
RUN addgroup -S app && adduser -S -G app app

COPY --from=builder /out/server /usr/local/bin/server

USER app
EXPOSE 8080

# Se usa 127.0.0.1 y no `localhost`: en Alpine, `localhost` resuelve primero a
# ::1, mientras que el servidor hace bind en 0.0.0.0, que es solo IPv4. El
# chequeo fallaría con "connection refused" contra un servicio perfectamente sano.
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
