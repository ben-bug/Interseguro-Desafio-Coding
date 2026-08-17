import { useId, useState } from 'react';
import { ApiError, login } from '../api';

interface LoginPanelProps {
  onAuthenticated: (token: string) => void;
}

/**
 * Pantalla de acceso.
 *
 * El token que devuelve la API se guarda en memoria (estado de React) y no en
 * localStorage: un token ahí queda accesible a cualquier script de la página, de
 * modo que un XSS bastaría para robarlo. El costo es que recargar obliga a
 * entrar de nuevo, aceptable para una sesión de quince minutos.
 */
export function LoginPanel({ onAuthenticated }: LoginPanelProps) {
  const userId = useId();
  const passwordId = useId();

  const [username, setUsername] = useState('demo');
  const [password, setPassword] = useState('');
  const [revealed, setRevealed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setPending(true);
    setError(null);

    try {
      const response = await login(username, password);
      onAuthenticated(response.token);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : 'No se pudo iniciar sesión.');
      setPending(false);
    }
  };

  return (
    <div className="gate">
      <form className="gate__card" onSubmit={submit} noValidate>
        <div className="gate__intro">
          <h2 className="gate__title">
            {/* El saludo va en el color de acción y el resto en el azul
                institucional: el mismo recurso de dos tonos en una sola línea
                que usa Interseguro en su acceso. */}
            <span className="gate__greeting">¡Hola!</span> Inicia tu sesión
          </h2>
          <p className="gate__lead">Ingresa tus credenciales para continuar</p>
        </div>

        <div className="control">
          {/* La etiqueta existe para los lectores de pantalla aunque no se vea:
              un campo identificado solo por su marcador de posición se queda sin
              nombre en cuanto el usuario empieza a escribir. */}
          <label className="visually-hidden" htmlFor={userId}>
            Usuario
          </label>
          <input
            id={userId}
            className="control__input"
            type="text"
            placeholder="Usuario"
            autoComplete="username"
            autoFocus
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            required
          />
        </div>

        <div className="control">
          <label className="visually-hidden" htmlFor={passwordId}>
            Contraseña
          </label>
          <input
            id={passwordId}
            className="control__input control__input--with-action"
            type={revealed ? 'text' : 'password'}
            placeholder="Contraseña"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
          <button
            className="control__action"
            type="button"
            onClick={() => setRevealed((value) => !value)}
            // Describe lo que hará al pulsarse, no el estado en que está.
            aria-label={revealed ? 'Ocultar la contraseña' : 'Mostrar la contraseña'}
            aria-pressed={revealed}
          >
            {revealed ? <EyeOffIcon /> : <EyeIcon />}
          </button>
        </div>

        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}

        <button className="button button--primary button--block button--lg" type="submit" disabled={pending}>
          {pending ? 'Verificando…' : 'Ingresar'}
        </button>

        <p className="gate__note">
          {/* No se escribe la contraseña: se define por entorno y cambia en cada
              despliegue, así que dejarla aquí quedaría desactualizada y daría la
              impresión de ser un valor fijo del código. */}
          Credenciales de demostración: se definen en <code>DEMO_USERNAME</code> y{' '}
          <code>DEMO_PASSWORD</code> del archivo <code>.env</code>.
        </p>
      </form>
    </div>
  );
}

/* Iconos en línea: heredan el color del tema y no añaden peticiones de red. */

function EyeIcon() {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M2.2 12S5.8 5.5 12 5.5 21.8 12 21.8 12 18.2 18.5 12 18.5 2.2 12 2.2 12z" />
      <circle cx="12" cy="12" r="3.2" />
    </svg>
  );
}

function EyeOffIcon() {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M9.9 5.7A9.6 9.6 0 0 1 12 5.5c6.2 0 9.8 6.5 9.8 6.5a17 17 0 0 1-3.2 4.1M6.4 7.9A17 17 0 0 0 2.2 12S5.8 18.5 12 18.5c1.8 0 3.4-.5 4.7-1.3" />
      <path d="M10.1 10.2a3.2 3.2 0 0 0 4.4 4.4" />
      <path d="M3 3l18 18" />
    </svg>
  );
}
