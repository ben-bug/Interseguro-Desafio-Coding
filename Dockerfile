# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# 1. Build Frontend
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# ---------------------------------------------------------------------------
# 2. Build Node API
# ---------------------------------------------------------------------------
FROM node:22-alpine AS node-builder
WORKDIR /src/api-node
COPY api-node/package.json api-node/package-lock.json* ./
RUN npm install
COPY api-node/ ./
RUN npm run build
RUN npm prune --production

# ---------------------------------------------------------------------------
# 3. Build Go API
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS go-builder
WORKDIR /src/api-go
COPY api-go/go.mod api-go/go.sum* ./
RUN go mod download
COPY api-go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---------------------------------------------------------------------------
# 4. Final All-in-One Image
# ---------------------------------------------------------------------------
FROM node:22-alpine AS runtime

RUN apk add --no-cache nginx ca-certificates

# API Node
WORKDIR /app/api-node
COPY --from=node-builder /src/api-node/dist ./dist
COPY --from=node-builder /src/api-node/node_modules ./node_modules
COPY --from=node-builder /src/api-node/package.json ./package.json

# API Go
COPY --from=go-builder /out/server /usr/local/bin/server

# Frontend Web
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html

# Entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 80 8080 3000

CMD ["/entrypoint.sh"]
