import { useId } from 'react';
import type { Matrix } from '../api';

interface MatrixEditorProps {
  matrix: Matrix;
  onChange: (matrix: Matrix) => void;
  disabled: boolean;
}

/** Tope de la interfaz. La API acepta hasta 256, pero más allá de esto la grilla deja de ser editable a mano. */
const MAX_DIMENSION = 12;

/** Matrices de ejemplo, elegidas porque cada una muestra algo distinto del resultado. */
const EXAMPLES: Array<{ label: string; hint: string; matrix: Matrix }> = [
  {
    label: 'Clásica 3×3',
    hint: 'El ejemplo canónico de la literatura: R queda con diagonal 14, 175 y 35.',
    matrix: [
      [12, -51, 4],
      [6, 167, -68],
      [-4, 24, -41],
    ],
  },
  {
    label: 'Rectangular 4×2',
    hint: 'Más filas que columnas: se aprecia la diferencia entre la forma completa y la reducida.',
    matrix: [
      [1, 2],
      [3, 4],
      [5, 6],
      [7, 8],
    ],
  },
  {
    label: 'Identidad 3×3',
    hint: 'Q y R salen ambas diagonales, así que anyDiagonal queda en verdadero.',
    matrix: [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
    ],
  },
  {
    label: 'Rango deficiente',
    hint: 'Columnas linealmente dependientes: donde Gram-Schmidt se degrada y Householder no.',
    matrix: [
      [1, 2, 3],
      [2, 4, 6],
      [3, 6, 9],
    ],
  },
];

/** Editor de la matriz de entrada: dimensiones ajustables y celdas editables. */
export function MatrixEditor({ matrix, onChange, disabled }: MatrixEditorProps) {
  const rowsId = useId();
  const colsId = useId();

  const rows = matrix.length;
  const cols = matrix[0]?.length ?? 0;

  /** Redimensiona conservando los valores que caben en las nuevas dimensiones. */
  const resize = (nextRows: number, nextCols: number) => {
    const clampedRows = clamp(nextRows);
    const clampedCols = clamp(nextCols);

    onChange(
      Array.from({ length: clampedRows }, (_, i) =>
        Array.from({ length: clampedCols }, (_, j) => matrix[i]?.[j] ?? 0),
      ),
    );
  };

  const setCell = (i: number, j: number, raw: string) => {
    // Se acepta la coma como separador decimal: es lo que produce el teclado en
    // configuración regional española.
    const parsed = Number.parseFloat(raw.replace(',', '.'));
    const next = matrix.map((row) => [...row]);
    next[i][j] = Number.isFinite(parsed) ? parsed : 0;
    onChange(next);
  };

  return (
    <section className="editor" aria-labelledby="editor-title">
      <h2 id="editor-title" className="panel__title">
        Matriz de entrada
      </h2>

      <div className="editor__dims">
        <label className="field" htmlFor={rowsId}>
          <span className="field__label">Filas</span>
          <input
            id={rowsId}
            className="field__input"
            type="number"
            min={1}
            max={MAX_DIMENSION}
            value={rows}
            disabled={disabled}
            onChange={(event) => resize(Number(event.target.value), cols)}
          />
        </label>

        <span className="editor__times" aria-hidden="true">
          ×
        </span>

        <label className="field" htmlFor={colsId}>
          <span className="field__label">Columnas</span>
          <input
            id={colsId}
            className="field__input"
            type="number"
            min={1}
            max={MAX_DIMENSION}
            value={cols}
            disabled={disabled}
            onChange={(event) => resize(rows, Number(event.target.value))}
          />
        </label>
      </div>

      <div
        className="editor__grid"
        style={{ gridTemplateColumns: `repeat(${cols}, minmax(3.5rem, 1fr))` }}
      >
        {matrix.map((row, i) =>
          row.map((value, j) => (
            <input
              key={`${i}-${j}`}
              className="editor__cell"
              type="text"
              inputMode="decimal"
              value={value}
              disabled={disabled}
              aria-label={`Fila ${i + 1}, columna ${j + 1}`}
              onChange={(event) => setCell(i, j, event.target.value)}
              onFocus={(event) => event.target.select()}
            />
          )),
        )}
      </div>

      <div className="editor__examples">
        <span className="editor__examples-label">Ejemplos</span>
        <div className="editor__examples-list">
          {EXAMPLES.map((example) => (
            <button
              key={example.label}
              type="button"
              className="chip"
              title={example.hint}
              disabled={disabled}
              onClick={() => onChange(example.matrix.map((row) => [...row]))}
            >
              {example.label}
            </button>
          ))}
        </div>
      </div>
    </section>
  );
}

function clamp(value: number): number {
  if (!Number.isFinite(value)) return 1;
  return Math.min(Math.max(Math.trunc(value), 1), MAX_DIMENSION);
}
