/**
 * Persistencia de la sesión.
 *
 * El token se guarda en `sessionStorage` y no en `localStorage`: sobrevive a una
 * recarga, que es lo que espera cualquiera que pulse F5, pero desaparece al
 * cerrar la pestaña y no se comparte entre pestañas. Es el punto intermedio
 * razonable entre perder la sesión en cada recarga y dejar el token en disco de
 * forma indefinida.
 *
 * Conviene ser preciso sobre lo que esto protege y lo que no. Guardar el token
 * fuera de `localStorage` reduce la exposición —no queda a mano para
 * exfiltrarlo en bloque ni sobrevive al cierre de la pestaña—, pero no es una
 * defensa contra XSS: quien logre ejecutar código en la página puede leer este
 * mismo almacenamiento, o sencillamente usar la sesión activa desde ahí. La
 * defensa real contra XSS es no tener XSS.
 *
 * En un sistema en producción, lo correcto sería un token de acceso de vida
 * corta en memoria y un token de refresco en una cookie `httpOnly`, que el
 * JavaScript de la página no puede leer. Eso exige que la API emita y rote esas
 * cookies y protegerse de CSRF, superficie que hoy no existe y que excede el
 * alcance de este desafío.
 */

const STORAGE_KEY = 'qr-session-token';

/** Estructura mínima del payload de un JWT que aquí interesa. */
interface TokenPayload {
  /** Instante de expiración, en segundos desde la época Unix. */
  exp?: number;
}

/**
 * Devuelve el token guardado, o null si no hay o ya venció.
 *
 * Descartar aquí un token expirado es solo una cortesía de interfaz: evita
 * mostrar la aplicación durante un instante para que el primer request falle
 * con un 401. La comprobación que cuenta es la del servidor, que verifica la
 * firma; esta lee el payload sin validarlo y por sí sola no autoriza nada.
 */
export function readToken(): string | null {
  let token: string | null = null;
  try {
    token = sessionStorage.getItem(STORAGE_KEY);
  } catch {
    // Almacenamiento bloqueado (modo privado, política del navegador): se
    // trabaja sin sesión persistente en lugar de fallar.
    return null;
  }

  if (!token) return null;

  const expiresAt = expirationOf(token);
  if (expiresAt !== null && expiresAt <= Date.now()) {
    clearToken();
    return null;
  }

  return token;
}

/** Guarda el token para que sobreviva a una recarga de la página. */
export function storeToken(token: string): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, token);
  } catch {
    // Sin almacenamiento, la sesión vive solo en memoria: se pierde al
    // recargar, pero la aplicación sigue siendo utilizable.
  }
}

/** Borra el token al cerrar sesión o cuando el servidor lo rechaza. */
export function clearToken(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Nada que limpiar si no hay almacenamiento.
  }
}

/**
 * Lee el instante de expiración del payload, en milisegundos.
 *
 * Devuelve null si el token no tiene la forma esperada o no declara `exp`; en
 * ese caso se deja pasar y que decida el servidor, que es quien puede
 * comprobarlo de verdad.
 */
function expirationOf(token: string): number | null {
  const payload = token.split('.')[1];
  if (!payload) return null;

  try {
    // El payload viaja en base64url: se convierte al alfabeto que entiende atob
    // y se restituye el relleno que esa variante omite.
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=');

    const claims = JSON.parse(atob(padded)) as TokenPayload;
    return typeof claims.exp === 'number' ? claims.exp * 1000 : null;
  } catch {
    // Token ilegible: se descarta como si no existiera.
    return null;
  }
}
