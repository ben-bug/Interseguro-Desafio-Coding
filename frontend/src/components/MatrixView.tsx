import type { CSSProperties } from 'react';
import type { Matrix } from '../api';
import { formatValue, intensity, maxAbsolute } from '../format';

interface MatrixViewProps {
  matrix: Matrix;
  /** Símbolo de la matriz: A, Q o R. */
  symbol: string;
  /** Acento cromático. Q y R llevan colores distintos para leerse como piezas separadas. */
  accent: 'input' | 'q' | 'r';
  decimals: number;
  /**
   * Posición dentro de la ecuación (0 para A, 1 para Q, 2 para R). Retrasa el
   * arranque de la cascada para que las tres matrices no aparezcan a la vez,
   * sino en el orden en que se leen.
   */
  order?: number;
  /**
   * Atenúa los ceros bajo la diagonal principal. Solo se activa en R, donde esos
   * ceros son la consecuencia visible del algoritmo y no un dato más.
   */
  revealTriangle?: boolean;
}

/** Retraso entre matrices consecutivas de la ecuación. */
const MATRIX_STAGGER_MS = 190;
/** Retraso entre antidiagonales sucesivas dentro de una matriz. */
const DIAGONAL_STEP_MS = 26;
/** Tope del retraso interno: sin él, una matriz grande tardaría segundos. */
const MAX_INTERNAL_DELAY_MS = 460;
/**
 * A partir de este número de celdas la cascada deja de animarse.
 *
 * La API acepta matrices de hasta 256×256, es decir 65 536 celdas: animarlas
 * todas obligaría al navegador a componer decenas de miles de capas y la
 * aparición del resultado se volvería más lenta que el propio cálculo. Por
 * encima del umbral el resultado aparece de golpe, que es lo correcto cuando el
 * volumen de datos ya es el protagonista.
 */
const ANIMATION_CELL_LIMIT = 400;

/**
 * Dibuja una matriz con la notación de corchetes.
 *
 * El fondo de cada celda se sombrea según la magnitud del valor respecto del
 * mayor de esa matriz. Eso convierte la salida numérica en una imagen de la
 * estructura: en R el triángulo superior queda cargado y el inferior en blanco,
 * y en Q el sombreado muestra de inmediato si es diagonal.
 */
export function MatrixView({
  matrix,
  symbol,
  accent,
  decimals,
  order = 0,
  revealTriangle = false,
}: MatrixViewProps) {
  const scale = maxAbsolute(matrix);
  const rows = matrix.length;
  const cols = matrix[0]?.length ?? 0;
  const animated = rows * cols <= ANIMATION_CELL_LIMIT;

  return (
    <figure className={`matrix matrix--${accent}${animated ? ' matrix--animated' : ''}`}>
      <figcaption className="matrix__caption" style={{ animationDelay: `${order * MATRIX_STAGGER_MS}ms` }}>
        <span className="matrix__symbol">{symbol}</span>
        <span className="matrix__dims">
          {rows}×{cols}
        </span>
      </figcaption>

      <div className="matrix__frame">
        <span className="matrix__bracket matrix__bracket--left" aria-hidden="true" />

        <div
          className="matrix__grid"
          style={{ gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))` }}
          role="table"
          aria-label={`Matriz ${symbol} de ${rows} por ${cols}`}
        >
          {matrix.map((row, i) =>
            row.map((value, j) => {
              // Los ceros estructurales bajo la diagonal se atenúan en lugar de
              // ocultarse: la forma triangular se ve, pero el dato sigue ahí.
              const isStructuralZero = revealTriangle && i > j && value === 0;

              const style: CSSProperties = {
                // La cascada avanza por antidiagonales (i + j) y no fila por
                // fila: el frente de onda cruza la matriz en diagonal, que es la
                // misma dirección en la que Householder va dejando los ceros.
                animationDelay: `${
                  order * MATRIX_STAGGER_MS +
                  Math.min((i + j) * DIAGONAL_STEP_MS, MAX_INTERNAL_DELAY_MS)
                }ms`,
              };
              if (!isStructuralZero) {
                (style as Record<string, unknown>)['--cell-intensity'] = intensity(value, scale);
              }

              return (
                <span
                  key={`${i}-${j}`}
                  role="cell"
                  className={`matrix__cell${isStructuralZero ? ' matrix__cell--structural' : ''}`}
                  style={style}
                  // El valor completo queda accesible sin ocupar la grilla.
                  title={String(value)}
                >
                  {formatValue(value, decimals)}
                </span>
              );
            }),
          )}
        </div>

        <span className="matrix__bracket matrix__bracket--right" aria-hidden="true" />
      </div>
    </figure>
  );
}
