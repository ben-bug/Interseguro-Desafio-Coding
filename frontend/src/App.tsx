import { useState } from 'react';
import { ApiError, factorize, type Matrix, type QRResult } from './api';
import { LoginPanel } from './components/LoginPanel';
import { MatrixEditor } from './components/MatrixEditor';
import { MatrixView } from './components/MatrixView';
import { StatsPanel } from './components/StatsPanel';

/** Matriz inicial: el ejemplo canónico de la literatura sobre QR. */
const INITIAL_MATRIX: Matrix = [
  [12, -51, 4],
  [6, 167, -68],
  [-4, 24, -41],
];

type Mode = 'full' | 'reduced';

export function App() {
  const [token, setToken] = useState<string | null>(null);
  const [matrix, setMatrix] = useState<Matrix>(INITIAL_MATRIX);
  const [mode, setMode] = useState<Mode>('full');
  const [decimals, setDecimals] = useState(4);
  const [result, setResult] = useState<QRResult | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [pending, setPending] = useState(false);

  const run = async () => {
    if (!token) return;
    setPending(true);
    setError(null);

    try {
      setResult(await factorize(token, matrix, mode));
    } catch (cause) {
      setResult(null);
      setError(
        cause instanceof ApiError
          ? cause
          : new ApiError('UNKNOWN_ERROR', 'Ocurrió un error inesperado.'),
      );
      // Un token vencido no se puede recuperar reintentando: se vuelve al login.
      if (cause instanceof ApiError && cause.code === 'TOKEN_EXPIRED') {
        setToken(null);
      }
    } finally {
      setPending(false);
    }
  };

  if (!token) {
    return (
      <>
        <Masthead />
        <main className="shell">
          <LoginPanel
            onAuthenticated={(issued) => {
              setToken(issued);
              setResult(null);
              setError(null);
            }}
          />
        </main>
      </>
    );
  }

  return (
    <>
      <Masthead onSignOut={() => setToken(null)} />

      <main className="shell">
        <div className="layout">
          <div className="panel panel--input">
            <MatrixEditor matrix={matrix} onChange={setMatrix} disabled={pending} />

            <div className="controls">
              <fieldset className="segmented">
                <legend className="field__label">Forma</legend>
                {(['full', 'reduced'] as const).map((option) => (
                  <label key={option} className="segmented__option">
                    <input
                      type="radio"
                      name="mode"
                      value={option}
                      checked={mode === option}
                      disabled={pending}
                      onChange={() => setMode(option)}
                    />
                    <span>{option === 'full' ? 'Completa' : 'Reducida'}</span>
                  </label>
                ))}
              </fieldset>

              <label className="field">
                <span className="field__label">Decimales</span>
                <select
                  className="field__input"
                  value={decimals}
                  disabled={pending}
                  onChange={(event) => setDecimals(Number(event.target.value))}
                >
                  {[2, 4, 6, 10].map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <button
              className="button button--primary button--block"
              type="button"
              onClick={run}
              disabled={pending}
            >
              {pending ? 'Factorizando…' : 'Factorizar'}
            </button>

            {error && (
              <div className="alert" role="alert">
                <span className="alert__code">{error.code}</span>
                <span>{error.message}</span>
              </div>
            )}
          </div>

          <div className="panel panel--output">
            {result ? (
              <Result result={result} matrix={matrix} decimals={decimals} />
            ) : (
              <Placeholder />
            )}
          </div>
        </div>
      </main>
    </>
  );
}

function Masthead({ onSignOut }: { onSignOut?: () => void }) {
  return (
    <header className="masthead">
      <div className="masthead__brand">
        <span className="masthead__mark" aria-hidden="true">
          QR
        </span>
        <div>
          <h1 className="masthead__title">Factorización QR</h1>
          <p className="masthead__subtitle">Householder · Go + Fiber → Node + Express</p>
        </div>
      </div>

      {onSignOut && (
        <button className="button button--ghost" type="button" onClick={onSignOut}>
          Cerrar sesión
        </button>
      )}
    </header>
  );
}

/**
 * El resultado se presenta como la ecuación que representa, no como dos
 * tarjetas independientes: A = Q · R es la afirmación que el servicio acaba de
 * comprobar, y verla escrita hace evidente qué relación tienen las tres piezas.
 */
function Result({
  result,
  matrix,
  decimals,
}: {
  result: QRResult;
  matrix: Matrix;
  decimals: number;
}) {
  return (
    <>
      <div className="equation">
        <MatrixView matrix={matrix} symbol="A" accent="input" decimals={decimals} />
        <span className="equation__operator" aria-label="es igual a">
          =
        </span>
        <MatrixView matrix={result.q} symbol="Q" accent="q" decimals={decimals} />
        <span className="equation__operator" aria-label="multiplicado por">
          ·
        </span>
        <MatrixView
          matrix={result.r}
          symbol="R"
          accent="r"
          decimals={decimals}
          revealTriangle
        />
      </div>

      <dl className="meta">
        <div className="meta__item">
          <dt>Algoritmo</dt>
          <dd>{result.meta.algorithm}</dd>
        </div>
        <div className="meta__item">
          <dt>Forma</dt>
          <dd>{result.meta.mode === 'full' ? 'completa' : 'reducida'}</dd>
        </div>
        <div className="meta__item">
          {/* El residual es la prueba de que el resultado es correcto: mide
              cuánto se aleja Q·R de la matriz original. */}
          <dt title="Error relativo de reconstrucción ‖Q·R − A‖ / ‖A‖">Residual</dt>
          <dd>{result.meta.residual.toExponential(2)}</dd>
        </div>
        <div className="meta__item">
          <dt>Cálculo</dt>
          <dd>{result.meta.durationMs.toFixed(2)} ms</dd>
        </div>
      </dl>

      {result.statistics && <StatsPanel statistics={result.statistics} />}
    </>
  );
}

function Placeholder() {
  return (
    <div className="placeholder">
      <p className="placeholder__equation" aria-hidden="true">
        A = Q · R
      </p>
      <p className="placeholder__text">
        Define una matriz y pulsa <strong>Factorizar</strong>. La API Go la descompone en una matriz
        ortogonal Q y una triangular superior R, y la API Node calcula las estadísticas del
        resultado.
      </p>
    </div>
  );
}
