import type { Statistics } from '../api';
import { formatStat } from '../format';

interface StatsPanelProps {
  statistics: Statistics;
}

/** Etiquetas de las cinco medidas que pide el enunciado. */
const MEASURES = [
  { key: 'max', label: 'Máximo' },
  { key: 'min', label: 'Mínimo' },
  { key: 'average', label: 'Promedio' },
  { key: 'sum', label: 'Suma total' },
] as const;

/**
 * Resultado de la API Node.
 *
 * Se muestra primero el agregado sobre ambas matrices, que es lo que pide el
 * enunciado, y debajo el desglose por matriz, que responde la pregunta
 * inmediata siguiente: de cuál de las dos viene cada extremo.
 */
export function StatsPanel({ statistics }: StatsPanelProps) {
  const entries = Object.entries(statistics.perMatrix);

  return (
    <section className="stats" aria-labelledby="stats-title">
      <div className="stats__header">
        <h2 id="stats-title" className="panel__title">
          Estadísticas
        </h2>
      </div>

      <div className="stats__overall">
        {MEASURES.map((measure) => (
          <div key={measure.key} className="stat">
            <span className="stat__label">{measure.label}</span>
            <span className="stat__value">{formatStat(statistics.overall[measure.key])}</span>
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

      <div className="breakdown-scroll">
        <table className="breakdown">
          <caption className="breakdown__caption">Desglose por matriz</caption>
          <thead>
            <tr>
              <th scope="col">Matriz</th>
              <th scope="col">Máximo</th>
              <th scope="col">Mínimo</th>
              <th scope="col">Promedio</th>
              <th scope="col">Suma</th>
              <th scope="col">Diagonal</th>
            </tr>
          </thead>
          <tbody>
            {entries.map(([name, stats]) => (
              <tr key={name}>
                <th scope="row" className={`breakdown__name breakdown__name--${name}`}>
                  {name.toUpperCase()}
                </th>
                <td>{formatStat(stats.max)}</td>
                <td>{formatStat(stats.min)}</td>
                <td>{formatStat(stats.average)}</td>
                <td>{formatStat(stats.sum)}</td>
                <td>
                  <span
                    className={`flag${stats.isDiagonal ? ' flag--on' : ''}`}
                    // La tolerancia se deriva de la magnitud de cada matriz, así
                    // que difiere entre Q y R: exponerla hace auditable el juicio.
                    title={`Evaluado con una tolerancia de ${stats.tolerance.toExponential(2)}`}
                  >
                    {stats.isDiagonal ? 'Sí' : 'No'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
