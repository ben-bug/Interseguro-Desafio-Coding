# Despliegue en la nube

El proyecto admite dos formas de despliegue, y la elección depende de lo que ofrezca la plataforma:

| Forma | Archivo | Cuándo usarla |
| --- | --- | --- |
| **Todo en uno** | `Dockerfile` (raíz) | Plataformas que exponen un solo puerto público por servicio: Railway, Heroku, Koyeb. Las dos APIs y nginx conviven en un contenedor. |
| **Servicios separados** | `docker-compose.yml` | Cuando la plataforma permite varios servicios en red privada: Cloud Run, Render, Fly.io. Es la forma preferible: cada servicio escala y falla por separado. |

Todas las imágenes corren sin privilegios de root, declaran `HEALTHCHECK` y hacen apagado ordenado ante `SIGTERM`.

**La opción recomendada para este desafío es Railway** (opción A): es la de menor fricción, tiene plan gratuito y despliega desde el repositorio sin configuración adicional.

---

## Opción A — Railway (recomendada)

Railway detecta el `Dockerfile` de la raíz y construye la imagen todo en uno. El `railway.json` versionado ya deja configurado el chequeo de salud y la política de reinicio, de modo que no hay nada que ajustar en el panel más allá de las variables.

### Pasos

1. En [railway.app](https://railway.app), elegir **New Project → Deploy from GitHub repo** y seleccionar este repositorio.
2. Railway detecta el `Dockerfile` de la raíz automáticamente. No hace falta tocar la configuración de build.
3. **Definir las variables de entorno** en *Variables* antes del primer despliegue. Sin ellas el contenedor se niega a arrancar, a propósito:

   | Variable | Valor |
   | --- | --- |
   | `JWT_SECRET` | Generar con `openssl rand -base64 48` |
   | `DEMO_PASSWORD` | La contraseña de acceso que se quiera usar |

   El resto tiene valores por defecto razonables. `PORT` lo inyecta Railway; no hay que definirlo.

4. En *Settings → Networking*, pulsar **Generate Domain** para obtener la URL pública.

### Por qué el arranque falla si faltan las variables

Es deliberado. Un valor por defecto para `JWT_SECRET` sería un secreto presente en el repositorio: cualquiera que lo leyera podría firmar tokens válidos contra la instancia desplegada. El contenedor prefiere no levantar antes que levantar de forma insegura, y el log dice exactamente qué falta:

```
ERROR: falta definir la(s) variable(s) de entorno: JWT_SECRET DEMO_PASSWORD
Generar un secreto con:  openssl rand -base64 48
```

### Qué hace `railway.json`

```json
"healthcheckPath": "/health",      // Railway espera un 200 antes de enviar tráfico
"healthcheckTimeout": 60,          // margen para que arranquen los tres procesos
"restartPolicyType": "ON_FAILURE", // el entrypoint sale con código 1 si cae una API
"restartPolicyMaxRetries": 5       // evita el bucle infinito de reinicios
```

La política de reinicio funciona porque el `entrypoint.sh` vigila los tres procesos y hace caer el contenedor entero cuando uno muere. Sin esa supervisión, una API caída dejaría el contenedor «vivo» sirviendo 502 y Railway no lo reiniciaría nunca.

### Verificar el despliegue

Sustituyendo `TU-APP` por el dominio generado:

```bash
curl -s https://TU-APP.up.railway.app/health
```

```bash
curl -s -X POST https://TU-APP.up.railway.app/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"demo","password":"TU_DEMO_PASSWORD"}'
```

### Limitaciones de esta forma

Tres procesos en un contenedor es un compromiso con la plataforma, no un ideal de arquitectura. Comparten CPU y memoria, escalan juntos y sus logs se entremezclan en la misma salida. Cuando la plataforma permite servicios separados, `docker-compose.yml` refleja mejor el diseño: la API Node queda sin puerto público y solo la alcanza la API Go por la red interna.

---

## Las otras opciones

Los apartados siguientes despliegan **servicios separados**. Los comandos son ejecutables; requieren una cuenta y credenciales propias.

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

## Opción B — Google Cloud Run

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

## Opción C — Render

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

## Opción D — Fly.io

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
