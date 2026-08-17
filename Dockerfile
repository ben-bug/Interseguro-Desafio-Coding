# syntax=docker/dockerfile:1
#
# Imagen todo-en-uno para plataformas que exponen un solo puerto público por
# servicio (Railway, Heroku y similares). Empaqueta las dos APIs y el frontend
# tras un nginx.
#
# Para desplegar los servicios por separado —preferible cuando la plataforma lo
# permite— usar docker-compose.yml y los Dockerfile de cada subdirectorio.

# ---------------------------------------------------------------------------
# 1. Build del frontend
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend-builder
WORKDIR /src/frontend

# `npm ci` en lugar de `npm install`: instala exactamente las versiones del
# lockfile, de modo que el build es reproducible. `npm install` puede resolver
# versiones distintas y hacer que la imagen difiera de lo que se probó.
# `--ignore-scripts` evita ejecutar los scripts de postinstalación de las
# dependencias, superficie clásica de ataque de la cadena de suministro.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts

COPY frontend/ ./
RUN npm run build

# ---------------------------------------------------------------------------
# 2. Build de la API Node
# ---------------------------------------------------------------------------
FROM node:22-alpine AS node-builder
WORKDIR /src/api-node

COPY api-node/package.json api-node/package-lock.json ./
RUN npm ci --ignore-scripts

COPY api-node/ ./
RUN npm run build

# Se descartan las dependencias de desarrollo: la imagen final no necesita
# TypeScript, Vitest ni Supertest.
RUN npm prune --omit=dev

# ---------------------------------------------------------------------------
# 3. Build de la API Go
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS go-builder
WORKDIR /src/api-go

COPY api-go/go.mod api-go/go.sum ./
RUN go mod download

COPY api-go/ ./

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/socius/interseguro-challenge/api-go/internal/api.Version=${VERSION}" \
    -o /out/server ./cmd/server

# ---------------------------------------------------------------------------
# 4. Imagen final
# ---------------------------------------------------------------------------
FROM node:22-alpine AS runtime

RUN apk add --no-cache nginx ca-certificates dos2unix

# Usuario sin privilegios: si alguno de los tres procesos se ve comprometido,
# no corre como root.
#
# nginx necesita escribir su pid, sus archivos temporales y su configuración
# —que el entrypoint genera en tiempo de arranque, cuando ya conoce $PORT—, así
# que esos directorios pasan a ser propiedad del usuario. La directiva `user`
# de nginx.conf se elimina porque solo tiene efecto cuando el proceso maestro
# corre como root, y sin quitarla emite un aviso en cada arranque.
RUN addgroup -S app && adduser -S -G app app \
    && mkdir -p /run/nginx /var/lib/nginx/tmp /var/log/nginx /etc/nginx/http.d \
    && sed -i '/^user /d' /etc/nginx/nginx.conf \
    && chown -R app:app /run/nginx /var/lib/nginx /var/log/nginx /etc/nginx/http.d

# API Node
WORKDIR /app/api-node
COPY --from=node-builder /src/api-node/dist ./dist
COPY --from=node-builder /src/api-node/node_modules ./node_modules
COPY --from=node-builder /src/api-node/package.json ./package.json

# API Go
COPY --from=go-builder /out/server /usr/local/bin/server

# Frontend
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html

# dos2unix protege el arranque: si el archivo se versiona desde Windows puede
# llegar con finales de línea CRLF, y entonces la línea `#!/bin/sh\r` hace que
# el kernel no encuentre el intérprete. El .gitattributes del repositorio ya
# fuerza LF; esto es la segunda barrera.
COPY entrypoint.sh /entrypoint.sh
RUN dos2unix /entrypoint.sh && chmod +x /entrypoint.sh

USER app

# Puerto de referencia para ejecución local. En Railway y similares lo
# reemplaza la variable $PORT que inyecta la plataforma.
EXPOSE 8080

# La forma shell es intencional: $PORT solo se conoce en ejecución, y la forma
# exec no expandiría la variable.
HEALTHCHECK --interval=30s --timeout=5s --start-period=25s --retries=3 \
    CMD wget --quiet --spider "http://127.0.0.1:${PORT:-8080}/health" || exit 1

CMD ["/entrypoint.sh"]
