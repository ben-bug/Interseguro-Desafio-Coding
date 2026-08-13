# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Etapa de compilación
# ---------------------------------------------------------------------------
FROM node:24-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY tsconfig.json vite.config.ts index.html ./
COPY src ./src

RUN npm run build

# ---------------------------------------------------------------------------
# Etapa de ejecución: nginx sirve el build estático y hace de proxy a la API Go
# ---------------------------------------------------------------------------
FROM nginx:1.29-alpine

# La configuración por defecto se reemplaza por la del proyecto, que añade el
# proxy hacia la API y el fallback de la aplicación de página única.
RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/app.conf

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

# 127.0.0.1 explícito: `localhost` resuelve primero a ::1 en Alpine y la
# directiva `listen 80` de nginx solo atiende IPv4.
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:80/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
