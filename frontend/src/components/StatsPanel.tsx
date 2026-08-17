import type { MatrixStats, Statistics } from '../api';
import { formatStat } from '../format';

interface StatsPanelProps {
  statistics: Statistics;
}

/** Las cuatro medidas agregadas que pide el enunciado, en orden de lectura. */
const MEASURES = [
  { key: 'max', label: 'Máximo' },
  { key: 'min', label: 'Mínimo' },
  { key: 'average', label: 'Promedio' },
  { key: 'sum', label: 'Suma total' },
] as const;

/** Columnas del desglose. Se declaran como datos para no repetir el marcado. */
const BREAKDOWN_COLUMNS = [
  { key: 'max', label: 'Máximo' },
  { key: 'min', label: 'Mínimo' },
  { key: 'average', label: 'Promedio' },
  { key: 'sum', label: 'Suma' },
] as const satisfies ReadonlyArray<{ key: keyof MatrixStats; label: string }>;

/**
 * Resultado que devuelve la API Node.
 *
 * Primero el agregado sobre todas las matrices, que es lo que pide el enunciado,
 * y debajo el desglose por matriz, que responde la pregunta inmediata siguiente:
 * de cuál de las dos viene cada extremo.
 */
export function StatsPanel({ statistics }: StatsPanelProps) {
  const entries = Object.entries(statistics.perMatrix);

  return (
    <section className="stats" aria-labelledby="stats-title">
      <div className="stats__header">
        <h2 id="stats-title" className="panel__title">
          Estadísticas
        </h2>
        <span className="stats__origin">calculadas por la API Node</span>
      </div>

      <div className="stats__overall">
        {MEASURES.map(({ key, label }) => (
          <div key={key} className="stat">
            <span className="stat__label">{label}</span>
            <span className="stat__value">{formatStat(statistics.overall[key])}</span>
          </div>
        ))}
        <div className="stat">
          <span className="stat__label">Valores</span>
          <span className="stat__value">{statistics.overall.count}</span>
        </div>
      </div>

      <div className={`verdict${statistics.anyDiagonal ? ' verdict--yes' : ''}`}>
        <span className="verdict__question">¿Alguna matriz es diagonal?</span>
        <span className="verdict__answer">{statistics.anyDiagonal ? 'Sí' : 'No'}</span>
      </div>

      <Breakdown entries={entries} />
    </section>
  );
}

/**
 * Tabla de estadísticas por matriz.
 *
 * Con seis columnas no cabe en la pantalla de un teléfono, así que se desplaza
 * dentro de su propio contenedor en lugar de ensanchar la página. Ese contenedor
 * lleva `tabIndex` a propósito: un área desplazable que no puede recibir el foco
 * es inalcanzable para quien navega solo con teclado, que no tiene forma de
 * moverla. Con foco, las flechas la recorren.
 */
function Breakdown({ entries }: { entries: Array<[string, MatrixStats]> }) {
  return (
    <div
      className="breakdown-scroll"
      tabIndex={0}
      role="region"
      aria-labelledby="breakdown-caption"
    >
      <table className="breakdown">
        <caption id="breakdown-caption" className="breakdown__caption">
          Desglose por matriz
        </caption>
        <thead>
          <tr>
            <th scope="col">Matriz</th>
            {BREAKDOWN_COLUMNS.map(({ key, label }) => (
              <th key={key} scope="col">
                {label}
              </th>
            ))}
            <th scope="col">Diagonal</th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, stats]) => (
            <tr key={name}>
              <th scope="row" className={`breakdown__name breakdown__name--${name}`}>
                {name.toUpperCase()}
              </th>
              {BREAKDOWN_COLUMNS.map(({ key }) => (
                <td key={key}>{formatStat(stats[key] as number)}</td>
              ))}
              <td>
                <DiagonalFlag stats={stats} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Veredicto de diagonalidad de una matriz.
 *
 * Expone la tolerancia con que se evaluó: se deriva de la magnitud de cada
 * matriz por separado, de modo que difiere entre Q y R, y mostrarla convierte el
 * juicio en algo auditable en vez de en un dato que hay que creer.
 */
function DiagonalFlag({ stats }: { stats: MatrixStats }) {
  const verdict = stats.isDiagonal ? 'Sí' : 'No';

  return (
    <span
      className={`flag${stats.isDiagonal ? ' flag--on' : ''}`}
      title={`Evaluado con una tolerancia de ${stats.tolerance.toExponential(2)}`}
    >
      {verdict}
    </span>
  );
}
