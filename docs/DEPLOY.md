# Despliegue en la nube

Las tres imágenes son autocontenidas, corren sin privilegios de root, declaran `HEALTHCHECK` y hacen apagado ordenado ante `SIGTERM`. Sirven en cualquier plataforma que ejecute contenedores.

Este documento cubre tres opciones. Los comandos son ejecutables; requieren una cuenta y credenciales propias.

---

## Antes de desplegar, en cualquier plataforma

### 1. Generar un secreto real

El `JWT_SECRET` de `.env.example` es un marcador de posición. Generar uno nuevo:

```bash
openssl rand -base64 48
```

Debe ser **idéntico** en ambas APIs: la Go firma los tokens y la Node los verifica. Cárgalo en el gestor de secretos de la plataforma (Secret Manager, Render Environment Groups, `fly secrets`), nunca en un archivo versionado ni en el `Dockerfile`.

### 2. Restringir CORS

La API Go acepta cualquier origen para facilitar la evaluación. En `api-go/internal/api/router.go`, cambiar:

```go
AllowOrigins: []string{"*"},
```

por el dominio real del frontend.

### 3. Decidir la exposición de la API Node

En Docker Compose la API Node no publica puerto: solo la alcanza la API Go por la red interna. Conviene conservar esa propiedad en la nube — cada plataforma lo resuelve distinto y se indica más abajo.

---

## Opción A — Google Cloud Run

La mejor opción para este sistema: escala a cero cuando no hay tráfico, cobra por request y acepta contenedores sin cambios.

**Requisitos:** `gcloud` CLI y un proyecto con facturación activa.

```bash
gcloud auth login && gcloud config set project TU_PROYECTO
```

Habilitar los servicios necesarios:

```bash
gcloud services enable run.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com
```

Crear el secreto compartido:

```bash
openssl rand -base64 48 | gcloud secrets create jwt-secret --data-file=-
```

Construir y publicar las imágenes (Cloud Build compila en la nube, sin Docker local):

```bash
gcloud builds submit ./api-node --tag gcr.io/TU_PROYECTO/interseguro-api-node
```

```bash
gcloud builds submit ./api-go --tag gcr.io/TU_PROYECTO/interseguro-api-go
```

Desplegar la API Node **sin acceso público**, de modo que solo la alcance la API Go:

```bash
gcloud run deploy interseguro-api-node --image gcr.io/TU_PROYECTO/interseguro-api-node --region us-central1 --no-allow-unauthenticated --port 3000 --set-secrets JWT_SECRET=jwt-secret:latest
```

Anotar la URL que devuelve y desplegar la API Go apuntando a ella:

```bash
gcloud run deploy interseguro-api-go --image gcr.io/TU_PROYECTO/interseguro-api-go --region us-central1 --allow-unauthenticated --port 8080 --set-secrets JWT_SECRET=jwt-secret:latest,DEMO_PASSWORD=demo-password:latest --set-env-vars STATS_API_URL=https://URL-DE-API-NODE
```

Como la API Node quedó privada, hay que autorizar a la API Go a invocarla. Se le asigna una cuenta de servicio con el rol `roles/run.invoker` sobre el servicio Node:

```bash
gcloud run services add-iam-policy-binding interseguro-api-node --region us-central1 --member serviceAccount:LA-CUENTA-DE-API-GO --role roles/run.invoker
```

> **Ajuste necesario:** con la API Node privada, Cloud Run exige un token de identidad de Google en cada llamada, además del JWT de la aplicación. Habría que añadir al cliente de `api-go/internal/client/stats.go` la obtención de ese token desde el servidor de metadatos y enviarlo en el encabezado. La alternativa, más simple para una demostración, es desplegar ambos servicios con `--allow-unauthenticated` y confiar la protección al JWT propio, que ya cubre todos los endpoints salvo los de salud.

**Frontend:** el `nginx.conf` hace proxy a `http://api-go:8080`, nombre que solo existe dentro de la red de Docker Compose. Antes de construir la imagen hay que reemplazarlo por la URL pública de la API Go, o servir el build estático en Firebase Hosting o Cloud Storage y apuntar el frontend a esa URL.

---

## Opción B — Render

La opción con menos fricción: se conecta al repositorio y despliega solo. Tiene plan gratuito.

Requiere el repositorio en GitHub. Crear en la raíz un `render.yaml`:

```yaml
services:
  - type: web
    name: interseguro-api-node
    runtime: docker
    dockerfilePath: ./api-node/Dockerfile
    dockerContext: ./api-node
    healthCheckPath: /health
    envVars:
      - key: JWT_SECRET
        generateValue: true   # Render lo genera y lo comparte con el grupo
      - key: NODE_API_PORT
        value: 3000

  - type: web
    name: interseguro-api-go
    runtime: docker
    dockerfilePath: ./api-go/Dockerfile
    dockerContext: ./api-go
    healthCheckPath: /health
    envVars:
      - key: JWT_SECRET
        fromService:
          name: interseguro-api-node
          type: web
          envVarKey: JWT_SECRET
      - key: STATS_API_URL
        fromService:
          name: interseguro-api-node
          type: web
          property: hostport
      - key: DEMO_PASSWORD
        sync: false           # se define a mano en el panel

  - type: web
    name: interseguro-frontend
    runtime: docker
    dockerfilePath: ./frontend/Dockerfile
    dockerContext: ./frontend
```

Luego, en el panel de Render: **New → Blueprint**, seleccionar el repositorio y confirmar. `fromService` resuelve la URL de la API Node y comparte el secreto sin escribirlo en ninguna parte.

Sigue haciendo falta ajustar el `proxy_pass` del `nginx.conf` a la URL pública de la API Go.

---

## Opción C — Fly.io

Útil si interesa desplegar cerca de los usuarios o mantener las instancias siempre activas.

```bash
fly auth login
```

Cada servicio necesita su propio `fly.toml`. Para la API Node, en `api-node/fly.toml`:

```toml
app = "interseguro-api-node"
primary_region = "scl"

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 3000
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true

[[http_service.checks]]
  path = "/health"
  interval = "15s"
  timeout = "3s"
```

Desplegar y cargar el secreto:

```bash
cd api-node && fly launch --no-deploy && fly secrets set JWT_SECRET="$(openssl rand -base64 48)" && fly deploy
```

Para la API Go, el `fly.toml` es análogo con `internal_port = 8080`. La ventaja de Fly es la red privada: los servicios de una misma organización se alcanzan por `.internal`, de modo que la API Node no necesita exponerse a internet.

```bash
cd api-go && fly secrets set JWT_SECRET="EL-MISMO-DE-ARRIBA" DEMO_PASSWORD="..." STATS_API_URL="http://interseguro-api-node.internal:3000" && fly deploy
```

---

## Lista de verificación previa a producción

- [ ] `JWT_SECRET` generado con entropía real y guardado en el gestor de secretos de la plataforma.
- [ ] `DEMO_PASSWORD` cambiado, o sustituido el login de demostración por usuarios reales.
- [ ] `AllowOrigins` de CORS restringido al dominio del frontend.
- [ ] `proxy_pass` del `nginx.conf` apuntando a la URL real de la API Go.
- [ ] La API Node no accesible desde internet, o protegida por la red privada de la plataforma.
- [ ] `MAX_MATRIX_DIMENSION` ajustado a la memoria disponible en la instancia.
- [ ] Alertas configuradas sobre las líneas de log con `level: error`.
- [ ] `/health` configurado como chequeo de vitalidad y `/health/ready` como chequeo de disponibilidad.
