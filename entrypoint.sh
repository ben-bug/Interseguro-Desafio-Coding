#!/bin/sh
# ---------------------------------------------------------------------------
# Arranque de la imagen todo-en-uno (Railway y similares).
#
# Levanta los tres procesos —API Node, API Go y nginx— dentro de un mismo
# contenedor, porque estas plataformas exponen un único puerto público por
# servicio. El despliegue con servicios separados sigue estando en
# docker-compose.yml y es el preferible cuando la plataforma lo permite.
# ---------------------------------------------------------------------------
set -eu

# --- Secretos: sin valores por defecto -------------------------------------
#
# Un fallback aquí sería un secreto publicado en el repositorio: cualquiera que
# lo leyera podría firmar tokens válidos contra la instancia desplegada. Ambas
# APIs se niegan a arrancar sin estas variables, y este script mantiene esa
# garantía en lugar de anularla.
missing=""
[ -n "${JWT_SECRET:-}" ] || missing="${missing} JWT_SECRET"
[ -n "${DEMO_PASSWORD:-}" ] || missing="${missing} DEMO_PASSWORD"

if [ -n "${missing}" ]; then
    echo "ERROR: falta definir la(s) variable(s) de entorno:${missing}" >&2
    echo "" >&2
    echo "Configurarlas en el panel de la plataforma antes de desplegar." >&2
    echo "Generar un secreto con:  openssl rand -base64 48" >&2
    echo "JWT_SECRET debe ser el mismo valor para ambas APIs." >&2
    exit 1
fi

# --- Configuración con valores por defecto seguros --------------------------
export JWT_ISSUER="${JWT_ISSUER:-interseguro-qr-api}"
export JWT_AUDIENCE="${JWT_AUDIENCE:-interseguro-clients}"
export JWT_TTL_MINUTES="${JWT_TTL_MINUTES:-15}"
export DEMO_USERNAME="${DEMO_USERNAME:-demo}"

export NODE_ENV="${NODE_ENV:-production}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export MAX_MATRICES="${MAX_MATRICES:-16}"
export MAX_MATRIX_DIMENSION="${MAX_MATRIX_DIMENSION:-256}"

# Puertos internos fijos: solo se usan dentro del contenedor y se mantienen
# lejos del $PORT que asigna la plataforma para evitar colisiones.
export NODE_API_PORT="13000"
export GO_API_PORT="18080"
export STATS_API_URL="http://127.0.0.1:13000"
export STATS_API_TIMEOUT_SECONDS="${STATS_API_TIMEOUT_SECONDS:-5}"
export STATS_API_MAX_RETRIES="${STATS_API_MAX_RETRIES:-1}"

# Puerto público: lo inyecta la plataforma; 8080 al ejecutar en local.
export PORT="${PORT:-8080}"

# --- Configuración de nginx -------------------------------------------------
# Se genera en tiempo de arranque porque $PORT no se conoce hasta este momento.
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

    # Ambos chequeos se delegan a la API Go: /health responde mientras el
    # proceso viva, y /health/ready además verifica que la API Node responda.
    location /health {
        proxy_pass http://127.0.0.1:18080/health;
        proxy_http_version 1.1;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;

        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        client_max_body_size 16m;
    }

    location / {
        add_header Cache-Control "no-cache";
        try_files \$uri \$uri/ /index.html;
    }
}
EOF

# --- Arranque de los procesos ----------------------------------------------
echo "==> Iniciando API Node (estadísticas) en el puerto ${NODE_API_PORT}…"
cd /app/api-node && node dist/server.js &
NODE_PID=$!

echo "==> Iniciando API Go (factorización QR) en el puerto ${GO_API_PORT}…"
/usr/local/bin/server &
GO_PID=$!

echo "==> Iniciando nginx en el puerto ${PORT}…"
nginx -g "daemon off;" &
NGINX_PID=$!

# --- Apagado ordenado -------------------------------------------------------
# La plataforma envía SIGTERM al detener el contenedor. Se propaga a los tres
# procesos para que cierren sus conexiones en curso en lugar de morir de golpe.
terminate() {
    echo "==> Apagado solicitado, deteniendo los servicios…"
    kill -TERM "$NODE_PID" "$GO_PID" "$NGINX_PID" 2>/dev/null || true
    wait "$NODE_PID" "$GO_PID" "$NGINX_PID" 2>/dev/null || true
    exit 0
}
trap terminate INT TERM

# --- Supervisión ------------------------------------------------------------
#
# Esperar solo a nginx dejaría al contenedor "vivo" tras la caída de cualquiera
# de las dos APIs: seguiría respondiendo 502 de forma indefinida y el
# orquestador no lo reiniciaría, porque su proceso principal sigue en pie. Un
# fallo silencioso es peor que una caída limpia.
#
# Este bucle vigila los tres procesos y hace caer el contenedor entero en
# cuanto uno muera, con un código de salida distinto de cero para que la
# plataforma lo reinicie.
supervise() {
    name=$1
    pid=$2

    if ! kill -0 "$pid" 2>/dev/null; then
        echo "ERROR: ${name} (pid ${pid}) terminó de forma inesperada." >&2
        echo "==> Deteniendo el resto de los servicios…" >&2
        kill -TERM "$NODE_PID" "$GO_PID" "$NGINX_PID" 2>/dev/null || true
        exit 1
    fi
}

while true; do
    supervise "la API Node" "$NODE_PID"
    supervise "la API Go" "$GO_PID"
    supervise "nginx" "$NGINX_PID"
    sleep 5 &
    # Se espera al sleep en segundo plano para que las señales se atiendan de
    # inmediato: un `sleep` en primer plano las dejaría en cola hasta terminar.
    wait $!
done
