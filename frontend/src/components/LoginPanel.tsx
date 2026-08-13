import { useId, useState } from 'react';
import { ApiError, login } from '../api';

interface LoginPanelProps {
  onAuthenticated: (token: string) => void;
}

/**
 * Pantalla de acceso.
 *
 * El token que devuelve la API se guarda en memoria (estado de React) y no en
 * localStorage: un token en localStorage queda accesible a cualquier script de
 * la página, de modo que un XSS bastaría para robarlo. El costo es que recargar
 * la página obliga a entrar de nuevo, aceptable para una sesión de 15 minutos.
 */
export function LoginPanel({ onAuthenticated }: LoginPanelProps) {
  const userId = useId();
  const passwordId = useId();

  const [username, setUsername] = useState('demo');
  const [password, setPassword] = useState('');
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
      <form className="gate__card" onSubmit={submit}>
        <h2 className="gate__title">Iniciar sesión</h2>
        <p className="gate__lead">
          Las APIs exigen un token JWT. La API Go lo emite al validar estas credenciales.
        </p>

        <label className="field" htmlFor={userId}>
          <span className="field__label">Usuario</span>
          <input
            id={userId}
            className="field__input field__input--wide"
            type="text"
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            required
          />
        </label>

        <label className="field" htmlFor={passwordId}>
          <span className="field__label">Contraseña</span>
          <input
            id={passwordId}
            className="field__input field__input--wide"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </label>

        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}

        <button className="button button--primary" type="submit" disabled={pending}>
          {pending ? 'Verificando…' : 'Entrar'}
        </button>

        <p className="gate__note">
          Credenciales de demostración: se definen en <code>DEMO_USERNAME</code> y{' '}
          <code>DEMO_PASSWORD</code> del archivo <code>.env</code>.
        </p>
      </form>
    </div>
  );
}
