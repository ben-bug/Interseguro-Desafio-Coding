/**
 * Cálculo de estadísticas sobre las matrices que devuelve la factorización QR.
 *
 * El módulo es lógica pura, sin dependencias de Express ni del entorno: recibe
 * matrices y devuelve un resultado. Eso permite probarlo exhaustivamente sin
 * levantar un servidor y reutilizarlo desde otro transporte si hiciera falta.
 */

/**
 * Factor de tolerancia relativa para decidir si un valor "es cero".
 *
 * La factorización QR produce residuos de redondeo: donde la matemática exacta
 * da 0, la aritmética de punto flotante deja valores del orden de 1e-16 veces
 * la escala de la matriz. Comparar con `=== 0` haría que ninguna matriz real
 * pareciera diagonal jamás.
 */
export const TOLERANCE_FACTOR = 1e-9;

/**
 * Piso absoluto de la tolerancia. Sin él, una matriz de valores minúsculos
 * (o la matriz nula) tendría tolerancia 0 y volveríamos a comparar exacto.
 */
export const MIN_TOLERANCE = 1e-12;

/** Estadísticas de una matriz individual. */
export interface MatrixStats {
  max: number;
  min: number;
  average: number;
  sum: number;
  /** Cantidad de elementos considerados (filas × columnas). */
  count: number;
  rows: number;
  cols: number;
  isSquare: boolean;
  isDiagonal: boolean;
  /** Tolerancia con la que se evaluó `isDiagonal`, para que el resultado sea auditable. */
  tolerance: number;
}

/** Estadísticas agregadas sobre el conjunto completo de matrices. */
export interface OverallStats {
  max: number;
  min: number;
  average: number;
  sum: number;
  count: number;
}

/** Resultado completo del cálculo. */
export interface StatisticsResult {
  overall: OverallStats;
  /** Desglose por matriz, en el mismo orden en que llegaron. */
  perMatrix: Record<string, MatrixStats>;
  /** True si al menos una de las matrices es diagonal. */
  anyDiagonal: boolean;
  /** Factor relativo usado para derivar la tolerancia de cada matriz. */
  toleranceFactor: number;
}

/**
 * Acumulador de suma con compensación de Neumaier.
 *
 * Sumar miles de valores de magnitudes distintas en punto flotante pierde
 * precisión: los términos pequeños se descartan al sumarse a un acumulador
 * grande. Este algoritmo conserva el error de cada paso en una variable de
 * compensación y lo reintegra al final, con un coste de tres operaciones extra
 * por elemento. Es la variante de Kahan-Babuška, que a diferencia del Kahan
 * clásico también funciona cuando el término entrante es mayor que la suma
 * acumulada, caso frecuente al mezclar los valores de Q (~1) con los de R.
 */
class CompensatedSum {
  #sum = 0;
  #compensation = 0;

  add(value: number): void {
    const total = this.#sum + value;
    this.#compensation +=
      Math.abs(this.#sum) >= Math.abs(value)
        ? this.#sum - total + value
        : value - total + this.#sum;
    this.#sum = total;
  }

  get value(): number {
    return this.#sum + this.#compensation;
  }
}

/**
 * Calcula las estadísticas de una matriz en un solo recorrido.
 *
 * Se asume que la matriz ya fue validada (rectangular, no vacía y con valores
 * finitos): la validación vive en la capa de esquema, no acá.
 */
export function computeMatrixStatistics(matrix: number[][]): MatrixStats {
  const rows = matrix.length;
  const cols = matrix[0].length;

  let max = Number.NEGATIVE_INFINITY;
  let min = Number.POSITIVE_INFINITY;
  let maxAbs = 0;
  const sum = new CompensatedSum();

  for (let i = 0; i < rows; i += 1) {
    const row = matrix[i];
    for (let j = 0; j < cols; j += 1) {
      const value = row[j];
      if (value > max) max = value;
      if (value < min) min = value;

      const absolute = Math.abs(value);
      if (absolute > maxAbs) maxAbs = absolute;

      sum.add(value);
    }
  }

  // La tolerancia se deriva de la escala de esta matriz en particular, no del
  // conjunto. Una tolerancia global tomada de la matriz de mayor magnitud haría
  // que en una matriz de valores pequeños se descartaran como "ruido" valores
  // que en su propia escala son perfectamente significativos.
  const tolerance = Math.max(MIN_TOLERANCE, TOLERANCE_FACTOR * maxAbs);
  const count = rows * cols;

  return {
    max,
    min,
    average: sum.value / count,
    sum: sum.value,
    count,
    rows,
    cols,
    isSquare: rows === cols,
    isDiagonal: isDiagonal(matrix, tolerance),
    tolerance,
  };
}

/**
 * Indica si todos los elementos fuera de la diagonal principal son cero dentro
 * de la tolerancia dada.
 *
 * Se usa la definición generalizada `a[i][j] ≈ 0 para todo i ≠ j`, que también
 * aplica a matrices rectangulares. La definición estricta de "matriz diagonal"
 * exige además que sea cuadrada; por eso el resultado se acompaña siempre del
 * campo `isSquare`, y quien necesite el criterio estricto puede exigir ambos.
 *
 * Casos límite, ambos correctos por la definición: la matriz nula es diagonal
 * (no tiene ningún elemento no nulo fuera de la diagonal) y una matriz de 1×1
 * también lo es (no tiene elementos fuera de la diagonal).
 */
export function isDiagonal(matrix: number[][], tolerance: number): boolean {
  for (let i = 0; i < matrix.length; i += 1) {
    const row = matrix[i];
    for (let j = 0; j < row.length; j += 1) {
      if (i !== j && Math.abs(row[j]) > tolerance) {
        return false;
      }
    }
  }
  return true;
}

/**
 * Calcula las estadísticas de cada matriz y las agregadas sobre el conjunto.
 *
 * El enunciado pide "el valor máximo encontrado en las matrices" (en plural),
 * de modo que `overall` recorre todas juntas. El desglose por matriz se entrega
 * además porque el recorrido ya está hecho y responde la pregunta natural
 * siguiente: cuál de las dos aporta cada extremo.
 */
export function computeStatistics(matrices: Record<string, number[][]>): StatisticsResult {
  const perMatrix: Record<string, MatrixStats> = {};

  let max = Number.NEGATIVE_INFINITY;
  let min = Number.POSITIVE_INFINITY;
  let count = 0;
  let anyDiagonal = false;
  const total = new CompensatedSum();

  for (const [name, matrix] of Object.entries(matrices)) {
    const stats = computeMatrixStatistics(matrix);
    perMatrix[name] = stats;

    // El agregado se compone a partir de los parciales en lugar de recorrer las
    // matrices otra vez: max, min, suma y conteo son todos asociativos.
    if (stats.max > max) max = stats.max;
    if (stats.min < min) min = stats.min;
    total.add(stats.sum);
    count += stats.count;
    anyDiagonal ||= stats.isDiagonal;
  }

  return {
    overall: {
      max,
      min,
      average: total.value / count,
      sum: total.value,
      count,
    },
    perMatrix,
    anyDiagonal,
    toleranceFactor: TOLERANCE_FACTOR,
  };
}
