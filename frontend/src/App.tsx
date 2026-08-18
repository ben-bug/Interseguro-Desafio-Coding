import { useState } from 'react';
import { ApiError, factorize, type Matrix, type QRResult } from './api';
import { clearToken, readToken, storeToken } from './session';
import { LoginPanel } from './components/LoginPanel';
import { MatrixEditor } from './components/MatrixEditor';
import { MatrixView } from './components/MatrixView';
import { StatsPanel } from './components/StatsPanel';
import { ThemeToggle } from './components/ThemeToggle';
import { Wordmark } from './components/Wordmark';

/** Matriz inicial: el ejemplo canónico de la literatura sobre QR. */
const INITIAL_MATRIX: Matrix = [
  [12, -51, 4],
  [6, 167, -68],
  [-4, 24, -41],
];

type Mode = 'full' | 'reduced';

export function App() {
  // La sesión se restaura desde sessionStorage, de modo que recargar la página
  // no obliga a entrar de nuevo. Ver session.ts para el porqué de ese
  // almacenamiento y sus límites.
  const [token, setToken] = useState<string | null>(() => readToken());
  const [matrix, setMatrix] = useState<Matrix>(INITIAL_MATRIX);
  const [mode, setMode] = useState<Mode>('full');
  const [decimals, setDecimals] = useState(4);
  const [result, setResult] = useState<QRResult | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [pending, setPending] = useState(false);
  // Cuenta las factorizaciones de la sesión. Solo sirve para distinguir la
  // primera —donde el marco entero aparece— de las siguientes, donde el marco
  // ya está en pantalla y solo cambian los datos.
  const [runCount, setRunCount] = useState(0);

  /**
   * Único punto por el que cambia la sesión. Centralizarlo evita el fallo
   * clásico de limpiar el estado y olvidar el almacenamiento, que dejaría un
   * token muerto sobreviviendo a la recarga.
   */
  const applySession = (next: string | null) => {
    setToken(next);
    if (next) {
      storeToken(next);
    } else {
      clearToken();
    }
  };

  const run = async () => {
    if (!token) return;
    setPending(true);
    setError(null);

    try {
      setResult(await factorize(token, matrix, mode));
      setRunCount((count) => count + 1);
    } catch (cause) {
      setResult(null);
      setError(
        cause instanceof ApiError
          ? cause
          : new ApiError('UNKNOWN_ERROR', 'Ocurrió un error inesperado.'),
      );
      // Un token vencido no se puede recuperar reintentando: se vuelve al
      // login y se descarta también el que estaba guardado.
      if (cause instanceof ApiError && cause.code === 'TOKEN_EXPIRED') {
        applySession(null);
      }
    } finally {
      setPending(false);
    }
  };

  if (!token) {
    return (
      <div className="login-page">
        <Masthead />
        <Hero />
        <main className="shell shell--login">
          <LoginPanel
            onAuthenticated={(issued) => {
              applySession(issued);
              setResult(null);
              setError(null);
            }}
          />
        </main>
      </div>
    );
  }

  return (
    <div className="workspace-page">
      <Masthead onSignOut={() => applySession(null)} />
      <Hero compact />

      <main className="shell shell--workspace">
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

          <div className={`panel panel--output${result ? '' : ' panel--placeholder'}`}>
            {result ? (
              <Result
                result={result}
                matrix={matrix}
                decimals={decimals}
                isFirstRun={runCount <= 1}
              />
            ) : (
              <Placeholder />
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

function Masthead({ onSignOut }: { onSignOut?: () => void }) {
  return (
    <header className="masthead">
      <Wordmark />

      {/* Las acciones de la cabecera se agrupan para que queden juntas al borde
          derecho y no se separen cuando falta el botón de salir. */}
      <div className="masthead__actions">
        <ThemeToggle />

        {onSignOut && (
          <button className="button button--ghost" type="button" onClick={onSignOut}>
          {/* La etiqueta se acorta en pantallas angostas, donde el ancho de la
              cabecera es escaso; el texto completo sigue siendo el que leen los
              lectores de pantalla en cualquier tamaño. */}
            <span className="signout__long">Cerrar sesión</span>
            <span className="signout__short" aria-hidden="true">
              Salir
            </span>
          </button>
        )}
      </div>
    </header>
  );
}

/**
 * Encabezado de contenido bajo la barra de marca.
 *
 * Nombra la herramienta y lo que hace. En la pantalla de acceso se muestra
 * completo, porque ahí es lo único que explica dónde ha llegado el usuario; una
 * vez dentro se compacta a una línea, para no robarle altura al área de trabajo,
 * que es donde ocurre todo.
 */
function Hero({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`hero${compact ? ' hero--compact' : ''}`}>
      <div className="hero__inner">
        <p className="hero__eyebrow">Herramienta de cálculo</p>
        <h1 className="hero__title">Factorización QR de matrices</h1>
        {!compact && (
          <p className="hero__subtitle">
            Descompón cualquier matriz en sus factores Q y R, y obtén estadísticas del resultado
            en segundos.
          </p>
        )}
      </div>
    </div>
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
  isFirstRun,
}: {
  result: QRResult;
  matrix: Matrix;
  decimals: number;
  /** True solo en la primera factorización de la sesión. */
  isFirstRun: boolean;
}) {
  return (
    // La clave cambia con cada factorización, lo que fuerza a React a montar de
    // nuevo todo el bloque y reinicia la cascada desde el principio. Sin ella,
    // React reutilizaría los mismos nodos al recalcular y la animación solo se
    // vería la primera vez. Envuelve el resultado completo —ecuación, datos del
    // cálculo y estadísticas— para que la onda los recorra en un solo gesto.
    // Se usa el identificador del request y no el resultado entero, de modo que
    // cambiar los decimales no vuelve a animar: solo se anima cuando hay
    // números nuevos que mostrar.
    <div
      className={`result${isFirstRun ? ' result--first' : ''}`}
      key={result.meta.requestId ?? result.meta.durationMs}
    >
      <div className="equation">
        <MatrixView matrix={matrix} symbol="A" accent="input" decimals={decimals} order={0} />
        <span className="equation__operator" aria-label="es igual a" style={{ animationDelay: '150ms' }}>
          =
        </span>
        <MatrixView matrix={result.q} symbol="Q" accent="q" decimals={decimals} order={1} />
        <span className="equation__operator" aria-label="multiplicado por" style={{ animationDelay: '340ms' }}>
          ·
        </span>
        <MatrixView
          matrix={result.r}
          symbol="R"
          accent="r"
          decimals={decimals}
          order={2}
          revealTriangle
        />
      </div>

      <dl className="meta">
        <div className="meta__item">
          <dt>Método</dt>
          <dd style={{ textTransform: 'capitalize' }}>{result.meta.algorithm}</dd>
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
    </div>
  );
}

function Placeholder() {
  return (
    <div className="placeholder">
      <p className="placeholder__equation" aria-hidden="true">
        A = Q · R
      </p>
      <p className="placeholder__text">
        Ingresa una matriz y pulsa <strong>Factorizar</strong> para obtener su descomposición Q · R
        y las estadísticas asociadas al instante.
      </p>
    </div>
  );
}
