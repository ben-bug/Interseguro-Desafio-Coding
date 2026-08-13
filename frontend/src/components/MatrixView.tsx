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
   * Atenúa los ceros bajo la diagonal principal. Solo se activa en R, donde esos
   * ceros son la consecuencia visible del algoritmo y no un dato más.
   */
  revealTriangle?: boolean;
}

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
  revealTriangle = false,
}: MatrixViewProps) {
  const scale = maxAbsolute(matrix);
  const rows = matrix.length;
  const cols = matrix[0]?.length ?? 0;

  return (
    <figure className={`matrix matrix--${accent}`}>
      <figcaption className="matrix__caption">
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

              return (
                <span
                  key={`${i}-${j}`}
                  role="cell"
                  className={`matrix__cell${isStructuralZero ? ' matrix__cell--structural' : ''}`}
                  style={{
                    // El retraso escalonado por fila evoca el avance del
                    // algoritmo, que procesa una columna por vez.
                    animationDelay: `${Math.min(i * 45, 400)}ms`,
                    ...(isStructuralZero
                      ? {}
                      : { '--cell-intensity': intensity(value, scale) } as React.CSSProperties),
                  }}
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
