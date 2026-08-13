# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Etapa de compilación: TypeScript -> JavaScript
# ---------------------------------------------------------------------------
FROM node:24-alpine AS builder

WORKDIR /app

# El manifiesto se copia solo para instalar: mientras las dependencias no
# cambien, Docker reutiliza esta capa aunque el código sí cambie.
COPY package.json package-lock.json ./

# --ignore-scripts evita ejecutar los scripts de postinstalación de las
# dependencias, superficie clásica de ataque de la cadena de suministro.
# Ninguna dependencia de este proyecto los necesita para compilar con tsc.
RUN npm ci --ignore-scripts

COPY tsconfig.json tsconfig.build.json ./
COPY src ./src

RUN npm run build

# ---------------------------------------------------------------------------
# Etapa de ejecución
# ---------------------------------------------------------------------------
FROM node:24-alpine AS runtime

WORKDIR /app
ENV NODE_ENV=production

COPY package.json package-lock.json ./

# Solo dependencias de producción: la imagen final no lleva TypeScript, Vitest
# ni Supertest, que no aportan nada en ejecución y sí amplían la superficie.
RUN npm ci --omit=dev --ignore-scripts && npm cache clean --force

COPY --from=builder /app/dist ./dist

ARG VERSION=dev
ENV SERVICE_VERSION=${VERSION}

# La imagen oficial de Node ya trae el usuario `node` sin privilegios.
USER node
EXPOSE 3000

# Se usa 127.0.0.1 y no `localhost`: en Alpine, `localhost` resuelve primero a
# ::1, mientras que el servidor hace bind en 0.0.0.0, que es solo IPv4. El
# chequeo fallaría con "connection refused" contra un servicio perfectamente sano.
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:3000/health || exit 1

CMD ["node", "dist/server.js"]
