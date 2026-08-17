import { useEffect, useState } from 'react';

/**
 * Tema elegido explícitamente por el usuario.
 *
 * `null` significa que no ha elegido y manda la preferencia del sistema, que es
 * un estado distinto de «claro»: si el usuario cambia el tema de su equipo, la
 * aplicación debe seguirlo.
 */
type Theme = 'light' | 'dark';

/** Clave de almacenamiento. Se lee también desde el script de index.html. */
const STORAGE_KEY = 'qr-theme';

/**
 * Botón para alternar entre claro y oscuro.
 *
 * El tema se aplica escribiendo `data-theme` en <html>: la hoja de estilos
 * define cada color una sola vez con `light-dark()`, así que cambiar ese
 * atributo conmuta la paleta completa sin tocar ninguna regla.
 *
 * La elección se guarda en localStorage. Aquí es adecuado —es una preferencia
 * de presentación, no un dato sensible—, a diferencia del token de sesión, que
 * se mantiene solo en memoria.
 */
export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(() => currentTheme());

  // El sistema sigue mandando mientras el usuario no elija: si cambia el tema
  // del equipo con la aplicación abierta, esta lo acompaña. En cuanto hay una
  // elección explícita, este efecto deja de aplicarla.
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');

    const followSystem = (event: MediaQueryListEvent) => {
      if (readStoredTheme() === null) {
        setTheme(event.matches ? 'dark' : 'light');
      }
    };

    media.addEventListener('change', followSystem);
    return () => media.removeEventListener('change', followSystem);
  }, []);

  const toggle = () => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';

    setTheme(next);
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Modo privado o almacenamiento lleno: el tema se aplica igual, solo que
      // no sobrevive a la recarga. No es motivo para romper la interacción.
    }
  };

  const goingDark = theme === 'light';

  return (
    <button
      className="button button--icon"
      type="button"
      onClick={toggle}
      // El botón no cambia de nombre según el estado: describe lo que hará al
      // pulsarlo, que es lo que necesita saber quien lo escucha con un lector
      // de pantalla antes de decidir si activarlo.
      aria-label={goingDark ? 'Cambiar a tema oscuro' : 'Cambiar a tema claro'}
      title={goingDark ? 'Tema oscuro' : 'Tema claro'}
    >
      {goingDark ? <MoonIcon /> : <SunIcon />}
    </button>
  );
}

/** Tema activo al montar: el que el usuario eligió, o el del sistema. */
function currentTheme(): Theme {
  const stored = readStoredTheme();
  if (stored) return stored;

  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/** Lee la elección guardada, ignorando cualquier valor que no reconozca. */
function readStoredTheme(): Theme | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    return value === 'light' || value === 'dark' ? value : null;
  } catch {
    return null;
  }
}

/* Los iconos van como SVG en línea para heredar el color del tema y no añadir
   peticiones de red. `stroke-width` algo grueso los mantiene legibles a 18 px. */

function MoonIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </svg>
  );
}

function SunIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <circle cx="12" cy="12" r="4.2" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  );
}
