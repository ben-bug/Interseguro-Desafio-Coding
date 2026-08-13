# Decisiones de diseño

El enunciado pide explícitamente que, ante dudas, el candidato "tome decisiones informadas y las sustente". Este documento recoge esas decisiones y el razonamiento detrás de cada una.

---

## 1. Se implementó la factorización QR, no una rotación

**El problema.** El enunciado se contradice consigo mismo:

- En *Arquitectura*: «Esta API recibirá la matriz original como entrada, realizará **la rotación de la matriz** y luego enviará los datos resultantes a la segunda API».
- En *Funcionalidad requerida*: «Una API en Go que reciba como entrada un array de arrays de números que represente una matriz rectangular y devuelva **la factorización QR** de dicha matriz».

**La decisión.** Se implementó QR.

**Por qué.** La sección de funcionalidad es la normativa y la más específica: nombra la operación, el tipo de entrada y el tipo de salida. La palabra "rotación" aparece solo en la descripción general del flujo y encaja con un residuo de una versión anterior del enunciado, hipótesis que refuerza el hecho de que la operación adicional descrita más adelante habla de "las matrices" en plural —lo que produce QR (Q y R), no una rotación (una sola matriz).

Además, QR *es* una rotación en sentido geométrico: `Q` es una matriz ortogonal, y toda matriz ortogonal representa una rotación o una reflexión del espacio.

**La cobertura.** Para que la lectura alternativa no quede sin respuesta, se expuso también `POST /api/v1/rotate`, que hace la rotación de 90° clásica. Cuesta veinte líneas y elimina la ambigüedad por completo.

---

## 2. Reflexiones de Householder, no Gram-Schmidt

**Alternativas.** Gram-Schmidt clásico, Gram-Schmidt modificado, rotaciones de Givens, reflexiones de Householder.

**La decisión.** Householder.

**Por qué.** Gram-Schmidt clásico ortogonaliza cada columna restándole sus proyecciones sobre las anteriores. Cuando dos columnas están casi alineadas —una matriz mal condicionada—, esa resta cancela cifras significativas y la `Q` resultante deja de ser ortogonal, con un error que crece con el número de condición al cuadrado.

Householder no resta: refleja. Cada paso aplica una transformación ortogonal `H = I − 2vvᵀ`, y las transformaciones ortogonales preservan las normas, de modo que no amplifican el error de redondeo. El resultado es estable sin importar el condicionamiento de la entrada.

El test `TestQRIllConditioned` usa una matriz de Hilbert de 8×8, cuyo número de condición supera 1e10, y comprueba que `QᵀQ = I` se mantiene dentro de 1e-10.

Givens sería igual de estable, pero conviene cuando la matriz es dispersa o hay que actualizar una factorización existente; para matrices densas Householder hace menos operaciones.

**Sin librerías externas.** La implementación es propia. El ejercicio evalúa precisamente la capacidad de resolver el problema, y delegarlo en `gonum` no mostraría nada.

---

## 3. Factorización completa por defecto, reducida bajo demanda

**La decisión.** `mode=full` (por defecto) devuelve `Q` de m×m y `R` de m×n. `mode=reduced` devuelve, cuando m > n, `Q` de m×n y `R` de n×n.

**Por qué.** La forma completa es la definición canónica y está definida para cualquier matriz rectangular, incluidas las que tienen más columnas que filas. La reducida descarta las columnas de `Q` que solo multiplican el bloque nulo inferior de `R`: son irrelevantes para el producto pero sí forman parte de la base ortonormal completa, y quien resuelve mínimos cuadrados suele preferir la reducida por tamaño.

Ofrecer ambas cuesta un recorte de arrays y evita tener que adivinar cuál necesita quien consume la API. Cuando m ≤ n las dos formas coinciden.

---

## 4. Los ceros bajo la diagonal de R se fuerzan a cero exacto

**La decisión.** Tras el bucle de Householder, el triángulo inferior de `R` se rellena con ceros exactos.

**Por qué.** Matemáticamente esos elementos son cero por construcción. Lo que queda tras el cálculo es ruido de redondeo del orden de `ε·‖A‖`, es decir, valores como `3.5e-17`. Devolverlos obligaría a cada consumidor a decidir su propia tolerancia para responder «¿es esto triangular?», y el frontend mostraría una columna de números minúsculos donde la definición dice que hay ceros.

Es una limpieza justificada, no un maquillaje: el `residual` que acompaña a la respuesta se calcula **después** de esta limpieza, de modo que sigue midiendo el error real de la factorización que se está devolviendo.

---

## 5. "Matriz diagonal" con tolerancia relativa y por matriz

**El problema.** El enunciado pide «verificar si alguna matriz es diagonal», pero no dice qué hacer con dos complicaciones reales:

1. La factorización produce números de punto flotante. Comparar con `=== 0` haría que ninguna matriz calculada pareciera diagonal jamás.
2. La definición estricta exige una matriz cuadrada, y `R` puede ser rectangular.

**La decisión.**

- Se usa la definición generalizada: `a[i][j] ≈ 0` para todo `i ≠ j`, aplicable también a matrices rectangulares. La respuesta incluye siempre `isSquare`, de modo que quien necesite el criterio estricto puede exigir ambos campos.
- La comparación usa una tolerancia relativa: `tol = max(1e-12, 1e-9 · max|aᵢⱼ|)`, calculada **para cada matriz por separado**.

**Por qué por matriz y no global.** Es la parte menos obvia y la más importante. `Q` tiene valores del orden de 1 y `R`, en el ejemplo canónico, del orden de 175. Con una tolerancia global derivada de la mayor (1.75e-7), un elemento fuera de la diagonal de `Q` con valor 1e-8 se consideraría cero, cuando en la escala de `Q` ese valor es 10⁸ veces el épsilon de máquina: es un dato, no ruido. La matriz se reportaría como diagonal sin serlo. El test `no enmascara valores significativos de una matriz de magnitud pequeña` cubre exactamente ese escenario.

La tolerancia efectiva se devuelve en cada entrada de `perMatrix`, de modo que el juicio es auditable en vez de tener que confiarse.

**Casos límite, ambos correctos por definición:** la matriz nula es diagonal (no tiene ningún elemento no nulo fuera de la diagonal) y una matriz de 1×1 también lo es (no tiene elementos fuera de la diagonal).

---

## 6. Suma compensada de Neumaier

**La decisión.** Las sumas de la API Node usan un acumulador con compensación en lugar de `reduce((a, b) => a + b)`.

**Por qué.** Sumar valores de magnitudes muy distintas en punto flotante descarta los términos pequeños: `1e16 + 1` da `1e16` porque el resultado exacto no es representable. Al mezclar los valores de `Q` (orden 1) con los de `R` (orden 100+), el efecto es real aunque pequeño.

El algoritmo conserva el error de cada paso en una variable de compensación y lo reintegra al final, con un coste de tres operaciones extra por elemento. Se eligió la variante de Kahan-Babuška (Neumaier) sobre el Kahan clásico porque también funciona cuando el término entrante es **mayor** que la suma acumulada, que es justamente el caso al recorrer `Q` antes que `R`.

El test lo demuestra con `[1e16, 1, -1e16]`: la suma ingenua da 0, la compensada da 1.

---

## 7. Seguridad: el JWT del usuario se propaga entre servicios

**Alternativas.** (a) Proteger solo la API Go y dejar la Node abierta en la red interna. (b) Un token de máquina distinto para la comunicación entre servicios. (c) Propagar el token del usuario final.

**La decisión.** (c), con HS256 y un secreto compartido.

**Por qué.** La opción (a) apuesta todo a que nadie alcance la red interna; basta un contenedor comprometido para dejar la API Node completamente abierta. La (b) es más robusta en un sistema grande, pero pierde la identidad del usuario en el salto: los logs de la API Node no podrían decir quién originó la petición.

Propagar el token conserva la identidad de punta a punta y hace que ningún endpoint quede sin autenticar. El costo es que ambos servicios comparten el secreto, aceptable para dos servicios del mismo dominio desplegados juntos. En un sistema con más servicios convendría pasar a RS256, donde el emisor firma con la clave privada y cada consumidor verifica con la pública, sin que nadie más pueda emitir tokens.

**Detalles que importan:**

- El algoritmo se restringe explícitamente a HS256 en ambos servicios. Sin esa lista, un token con `alg: none` podría llegar a aceptarse. Hay un test para ello en cada API.
- Las credenciales del login se comparan en tiempo constante (`subtle.ConstantTimeCompare`), para que el tiempo de respuesta no filtre el prefijo correcto de la contraseña.
- El mensaje de error no distingue entre usuario inexistente y contraseña incorrecta, para no permitir enumerar usuarios.
- El frontend guarda el token en memoria y no en `localStorage`, donde un XSS bastaría para robarlo.

---

## 8. Los endpoints de salud son públicos, y hay dos

**La decisión.** `GET /health` (vitalidad) y `GET /health/ready` (disponibilidad) quedan fuera de la autenticación.

**Por qué públicos.** Los consultan el healthcheck de Docker, el balanceador y las plataformas cloud, ninguno de los cuales tiene credenciales. No exponen más que el nombre y la versión del servicio.

**Por qué dos.** `/health` no consulta dependencias: si lo hiciera, una caída de la API Node haría que el orquestador reiniciara la API Go, que está perfectamente sana, en vez de aislar el problema donde ocurre. `/health/ready` sí verifica el upstream, porque sin él el servicio no puede cumplir su función principal.

---

## 9. La autenticación se declara por ruta, no por grupo

**La decisión.** El middleware de JWT se registra en cada ruta protegida, en lugar de sobre un grupo que cubra todo `/api/v1`.

**Por qué.** Un grupo con prefijo vacío se comporta como un `Use()` sobre todo el espacio de rutas: interceptaría también las inexistentes, devolviendo 401 donde corresponde un 404, y haría que el carácter público o protegido de cada endpoint dependiera del orden en que se registró —de modo que añadir una ruta pública después del grupo la protegería sin querer.

Esto se detectó porque un test esperaba 404 en una ruta inexistente y recibió 401.

---

## 10. Los errores comparten formato entre ambas APIs

**La decisión.** Las dos APIs responden `{ "error": { "code", "message", "details", "requestId" } }` con el mismo catálogo de códigos para los mismos problemas.

**Por qué.** El frontend tiene un único camino de manejo de errores sin importar cuál de los dos servicios falló. Los códigos son estables y forman parte del contrato: el cliente ramifica sobre `code`, no sobre `message`, que está escrito para personas y puede cambiar.

Los errores esperados se registran en `warn` y los no contemplados en `error` con el stack completo, pero al cliente solo le llega un mensaje genérico: devolver el detalle interno filtraría rutas de archivos y estructura del código.

---

## 11. Límites de tamaño en ambas APIs

**La decisión.** Máximo 256 filas y 256 columnas por matriz, 16 matrices por petición y 16 MB de cuerpo. La API Node aplica sus propios límites en vez de confiar en que la Go ya validó.

**Por qué.** El costo de la factorización crece como O(m·n²): sin un límite, una sola petición con una matriz de 5000×5000 monopolizaría la CPU del servicio. Y validar en ambos extremos es lo correcto: la API Node es un servicio independiente y no puede asumir quién la llama.

---

## 12. TypeScript en la API Node

**Por qué.** El enunciado pide Node.js con Express, y TypeScript lo es. El tipado estático sobre las matrices y las respuestas detecta en compilación una clase de errores que de otro modo aparecerían en ejecución, y hace que el contrato entre servicios sea explícito y verificable.

La imagen de producción no lleva TypeScript: se compila en una etapa previa y la etapa final instala solo dependencias de ejecución.

---

## Qué haría distinto en producción

- **RS256 en lugar de HS256**, para que solo el emisor pueda firmar.
- **Usuarios reales** en lugar de credenciales de demostración en variables de entorno.
- **CORS restringido** por entorno; hoy acepta cualquier origen para facilitar la evaluación.
- **Límite de tasa** por IP y por sujeto, que hoy no existe.
- **Métricas** en formato Prometheus junto a los logs estructurados que ya se emiten.
- **Caché de resultados** si el patrón de uso mostrara matrices repetidas: la factorización es determinista y no tiene estado, así que se presta bien.
