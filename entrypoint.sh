#!/bin/sh
set -e

# Valores por defecto de entorno si no fueron pasados
export JWT_SECRET="${JWT_SECRET:-desafio-interseguro-secreto-local-no-usar-en-produccion}"
export JWT_ISSUER="${JWT_ISSUER:-interseguro-qr-api}"
export JWT_AUDIENCE="${JWT_AUDIENCE:-interseguro-clients}"
export JWT_TTL_MINUTES="${JWT_TTL_MINUTES:-15}"
export DEMO_USERNAME="${DEMO_USERNAME:-demo}"
export DEMO_PASSWORD="${DEMO_PASSWORD:-demo1234}"

export NODE_ENV="${NODE_ENV:-production}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export MAX_MATRICES="${MAX_MATRICES:-16}"
export MAX_MATRIX_DIMENSION="${MAX_MATRIX_DIMENSION:-256}"

export NODE_API_PORT="3000"
export GO_API_PORT="8080"
export STATS_API_URL="http://127.0.0.1:3000"
export STATS_API_TIMEOUT_SECONDS="${STATS_API_TIMEOUT_SECONDS:-5}"
export STATS_API_MAX_RETRIES="${STATS_API_MAX_RETRIES:-1}"

# Puerto asignado por Railway o fallback a 80/8080
export PORT="${PORT:-8080}"

# Configurar Nginx para escuchar en $PORT
mkdir -p /etc/nginx/http.d /run/nginx
cat << EOF > /etc/nginx/http.d/default.conf
server {
    listen $PORT;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    gzip on;
    gzip_types text/css application/javascript application/json image/svg+xml;
    gzip_min_length 1024;

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location / {
        add_header Cache-Control "no-cache";
        try_files \$uri \$uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        client_max_body_size 16m;
    }
}
EOF

# Iniciar Node API en segundo plano
echo "==> Iniciando API Node (estadísticas) en puerto 3000..."
cd /app/api-node && node dist/server.js &
NODE_PID=$!

# Iniciar Go API en segundo plano
echo "==> Iniciando API Go (QR) en puerto 8080..."
/usr/local/bin/server &
GO_PID=$!

cleanup() {
    echo "==> Apagando servicios..."
    kill -TERM "$NODE_PID" "$GO_PID" 2>/dev/null || true
    wait "$NODE_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
    exit 0
}
trap cleanup SIGINT SIGTERM

# Iniciar Nginx
echo "==> Iniciando Nginx en puerto $PORT..."
nginx -g "daemon off;" &
NGINX_PID=$!

wait "$NGINX_PID"
