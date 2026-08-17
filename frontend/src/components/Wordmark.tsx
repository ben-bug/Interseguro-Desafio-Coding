/**
 * Marca de la aplicación.
 *
 * Se dibuja con SVG en línea en lugar de cargar una imagen: hereda el color
 * del tema (así funciona igual en claro y en oscuro), escala sin perder nitidez
 * en cualquier densidad de pantalla y no añade una petición de red.
 */

/**
 * Símbolo: la silueta de una matriz triangular superior.
 *
 * Es la forma que deja el algoritmo al anular todo lo que hay bajo la diagonal,
 * de modo que el icono describe lo que la herramienta hace en vez de ser un
 * adorno. Las celdas anuladas se insinúan con opacidad baja para que el
 * triángulo se lea de inmediato.
 */
export function MatrixMark({ size = 30 }: { size?: number }) {
  return (
    <svg
      className="wordmark__symbol"
      width={size}
      height={size}
      viewBox="0 0 32 32"
      aria-hidden="true"
      focusable="false"
    >
      <rect width="32" height="32" rx="7" fill="currentColor" />
      <g fill="var(--surface)">
        <rect x="6" y="7" width="6" height="6" rx="1.5" />
        <rect x="14" y="7" width="6" height="6" rx="1.5" />
        <rect x="22" y="7" width="4" height="6" rx="1.5" />
        <rect x="14" y="15" width="6" height="6" rx="1.5" />
        <rect x="22" y="15" width="4" height="6" rx="1.5" />
        <rect x="22" y="23" width="4" height="4" rx="1.5" />
      </g>
      <g fill="var(--surface)" opacity="0.28">
        <rect x="6" y="15" width="6" height="6" rx="1.5" />
        <rect x="6" y="23" width="6" height="4" rx="1.5" />
        <rect x="14" y="23" width="6" height="4" rx="1.5" />
      </g>
    </svg>
  );
}

/** Símbolo y nombre, tal como aparecen en la cabecera. */
export function Wordmark() {
  return (
    <div className="wordmark">
      <MatrixMark />
      <div className="wordmark__text">
        <span className="wordmark__owner">Interseguro</span>
        <span className="wordmark__product">Cálculo QR</span>
      </div>
    </div>
  );
}
