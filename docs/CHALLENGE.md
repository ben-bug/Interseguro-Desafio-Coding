# Enunciado del desafío

Transcripción del PDF **«Coding Challenge — División TI, Junio 2024»**.

La primera parte reproduce el enunciado tal como está, sin corregir ni interpretar. La segunda indica dónde se resuelve cada punto en este repositorio. Se mantienen separadas para que se distinga lo que pide Interseguro de lo que afirma esta solución.

---

## Parte 1 — Transcripción del enunciado

### Consideraciones técnicas

- Utilizar el lenguaje de programación **Go (Golang)** para una API y **Node.js** para la otra API.
- Implementar la solución utilizando los frameworks **Fiber** para la API en Go y **Express.js** para la API en Node.js.
- Documentar el código de manera clara y concisa, siguiendo las mejores prácticas de codificación.
- Utilizar **Docker** para contenerizar las aplicaciones y facilitar su despliegue en diferentes entornos.
- Implementar la comunicación entre las dos API utilizando un mecanismo como **HTTP**.
  - Utilizar servicios en la nube para la implementación y el despliegue de las aplicaciones.

### Arquitectura de la solución

- **API en Go:** Esta API recibirá la matriz original como entrada, realizará la rotación de la matriz y luego enviará los datos resultantes a la segunda API en Node.js.
- **API en Node.js:** Esta API recibirá los datos de la matriz rotada de la API en Go, calculará estadísticas sobre los datos y devolverá estas estadísticas como resultado.

### Funcionalidad requerida

- Crear dos API RESTful:
  - Una API en Go que reciba como entrada un array de arrays de números que represente una matriz rectangular y devuelva la **factorización QR** de dicha matriz.
  - Otra API en Node.js que reciba el resultado de las matrices devueltas por la primera API y realice una operación adicional sobre los datos. (*) Detalle en la sección operaciones adicionales
- Implementar la lógica para realizar la rotación de la matriz y la operación adicional de manera eficiente y correcta en cada API.

### Funcionalidad opcional

- Implementar un frontend que consuma ambas APIs y muestre los resultados de la rotación de la matriz y las estadísticas adicionales.
- Aplicar un nivel de seguridad utilizando JWT para proteger las consultas a las APIs.
- Implementar pruebas unitarias y de integración para garantizar la calidad del código en ambas API.

### Operación adicional

La segunda API calculará lo siguientes sobre los datos de las matrices devueltas:

- **Valor máximo:** El valor máximo encontrado en las matrices.
- **Valor mínimo:** El valor mínimo encontrado en las matrices.
- **Promedio:** El promedio de todos los valores de las matrices.
- **Suma total:** La suma total de todos los valores de las matrices.
- **Matriz diagonal:** Verificar si alguna matriz es diagonal.

### Consideraciones

- No hay un estándar específico para los nombres de los objetos creados, pero se espera coherencia en su estructura y documentación.
- En caso de dudas en el enunciado, se espera que el candidato tome decisiones informadas y las sustente durante la entrevista.
- Se valorará la eficiencia y la elegancia de la solución implementada, así como la capacidad del candidato para comunicar y defender sus decisiones técnicas.

---

## Parte 2 — Dónde se resuelve cada punto

### La ambigüedad del enunciado

El documento se contradice, y como pide «tomar decisiones informadas y sustentarlas», la decisión se argumenta en lugar de zanjarse en silencio:

- **Arquitectura** dice que la API Go «realizará **la rotación** de la matriz».
- **Funcionalidad requerida** dice que devuelva «la **factorización QR** de dicha matriz».

Se implementó **QR**, por ser el requisito funcional explícito y el que concuerda con el resto del enunciado: pide un «array de arrays de números», estadísticas numéricas sobre el resultado y verificar si «alguna **matriz**» es diagonal, en plural — y QR devuelve dos matrices, mientras que una rotación devuelve una.

Para cubrir la lectura alternativa se expone además `POST /api/v1/rotate`, con la rotación de 90° clásica. El razonamiento completo está en la decisión 1 de [DECISIONS.md](DECISIONS.md).

### Requisitos técnicos

| Requisito | Dónde |
| --- | --- |
| Go con Fiber | [`api-go/`](../api-go) — Fiber v3 |
| Node.js con Express | [`api-node/`](../api-node) — Express 5 sobre TypeScript |
| Comunicación por HTTP | [`api-go/internal/client/stats.go`](../api-go/internal/client/stats.go) — con timeout, reintento y propagación del token |
| Docker | Un `Dockerfile` por servicio, `docker-compose.yml` para orquestarlos |
| Servicios en la nube | [`DEPLOY.md`](DEPLOY.md) — Railway, Cloud Run, Render y Fly.io |
| Documentación | Comentarios en el código, [`API.md`](API.md) y [`DECISIONS.md`](DECISIONS.md) |

### Funcionalidad requerida

| Requisito | Dónde |
| --- | --- |
| API Go: factorización QR | [`api-go/internal/matrix/qr.go`](../api-go/internal/matrix/qr.go) — reflexiones de Householder, sin librerías de álgebra lineal |
| API Node: operación adicional | [`api-node/src/services/statistics.service.ts`](../api-node/src/services/statistics.service.ts) |
| Rotación de la matriz | [`api-go/internal/matrix/rotate.go`](../api-go/internal/matrix/rotate.go) |

### Funcionalidad opcional

Las tres están implementadas.

| Opcional | Dónde |
| --- | --- |
| Frontend que consuma ambas APIs | [`frontend/`](../frontend) — React con Vite; presenta el resultado como la ecuación `A = Q · R` |
| Seguridad con JWT | [`api-go/internal/auth/jwt.go`](../api-go/internal/auth/jwt.go) y [`api-node/src/middleware/auth.ts`](../api-node/src/middleware/auth.ts) — HS256, con el token del usuario propagado entre servicios |
| Pruebas unitarias y de integración | 5 paquetes de test en Go y 65 pruebas en Node; cobertura por paquete en el [README](../README.md) |

### Operación adicional

Las cinco medidas se calculan en [`statistics.service.ts`](../api-node/src/services/statistics.service.ts) y se devuelven tanto agregadas sobre el conjunto como desglosadas por matriz.

| Medida | Nota de implementación |
| --- | --- |
| Valor máximo | En un solo recorrido junto con el resto |
| Valor mínimo | Íd. |
| Promedio | Derivado de la suma compensada |
| Suma total | Acumulador de Neumaier: sumar valores de magnitudes muy distintas con `+` pierde precisión |
| Matriz diagonal | Tolerancia relativa derivada de la magnitud de **cada** matriz. Comparar con `=== 0` haría que ninguna matriz calculada pareciera diagonal, porque QR deja residuos de redondeo. Ver la decisión 5 de [DECISIONS.md](DECISIONS.md) |
