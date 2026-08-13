/**
 * Formateo de números para la interfaz.
 *
 * La factorización devuelve valores como -0.857142857142857 y 1.6e-16. Volcarlos
 * crudos en la grilla la vuelve ilegible y esconde la estructura del resultado,
 * que es justo lo que se quiere ver.
 */

/** Umbral bajo el cual conviene la notación exponencial. */
const SMALL = 1e-4;
/** Umbral sobre el cual conviene la notación exponencial. */
const LARGE = 1e6;

/**
 * Formatea un valor con la cantidad de decimales pedida.
 *
 * Los valores muy pequeños o muy grandes pasan a notación exponencial: con
 * decimales fijos, 1.6e-16 se mostraría como "0.0000" y parecería un cero
 * exacto, que es precisamente la distinción que importa al leer una R.
 */
export function formatValue(value: number, decimals: number): string {
  if (Object.is(value, -0) || value === 0) return '0';
  if (!Number.isFinite(value)) return String(value);

  const magnitude = Math.abs(value);
  if (magnitude < SMALL || magnitude >= LARGE) {
    return value.toExponential(Math.min(decimals, 4));
  }
  return value.toFixed(decimals);
}

/** Formatea un valor para las tarjetas de estadísticas, siempre legible. */
export function formatStat(value: number): string {
  if (!Number.isFinite(value)) return '—';
  if (value === 0) return '0';

  const magnitude = Math.abs(value);
  if (magnitude < 1e-3 || magnitude >= 1e7) return value.toExponential(3);

  // Hasta cuatro decimales, sin ceros de relleno a la derecha.
  return Number(value.toFixed(4)).toString();
}

/**
 * Intensidad relativa de una celda, entre 0 y 1, usada para el sombreado que
 * revela la estructura de la matriz de un vistazo.
 *
 * La escala es la raíz cuadrada de la proporción: una escala lineal deja casi
 * invisible todo lo que no sea el máximo, porque en una matriz típica la mayoría
 * de los valores están muy por debajo del mayor.
 */
export function intensity(value: number, maxAbs: number): number {
  if (maxAbs === 0) return 0;
  return Math.sqrt(Math.min(Math.abs(value) / maxAbs, 1));
}

/** Mayor valor absoluto de la matriz, escala de referencia del sombreado. */
export function maxAbsolute(matrix: number[][]): number {
  let max = 0;
  for (const row of matrix) {
    for (const value of row) {
      const absolute = Math.abs(value);
      if (absolute > max) max = absolute;
    }
  }
  return max;
}
