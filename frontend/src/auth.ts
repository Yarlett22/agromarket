// Autenticación con Keycloak (OpenID Connect + PKCE).
// El usuario inicia sesión en Keycloak; la app recibe un access token JWT firmado
// con RS256 que envía al API Gateway en cada petición (Authorization: Bearer ...).
import Keycloak from 'keycloak-js';
import type { User } from './api';

export const keycloak = new Keycloak({
  url: `${window.location.origin}/auth`, // Keycloak publicado detrás del gateway (HTTPS + WAF)
  realm: 'agromarket',
  clientId: 'agromarket-web',
});

export async function initAuth(): Promise<User | null> {
  try {
    const authenticated = await keycloak.init({
      onLoad: 'check-sso',             // recupera la sesión si ya existe, sin forzar login
      pkceMethod: 'S256',              // Authorization Code + PKCE (sin secretos en el navegador)
      checkLoginIframe: false,
      silentCheckSsoRedirectUri: `${window.location.origin}/silent-check-sso.html`,
    });
    return authenticated ? currentUser() : null;
  } catch {
    return null; // Keycloak aún no disponible: la tienda funciona como invitado
  }
}

export function currentUser(): User | null {
  const t = keycloak.tokenParsed as Record<string, any> | undefined;
  if (!t) return null;
  return {
    id: t.sub,
    name: t.name || t.preferred_username,
    email: t.email,
    username: t.preferred_username,
    roles: (t.realm_access?.roles ?? []).filter((r: string) => ['cliente', 'admin'].includes(r)),
  };
}

// Devuelve un token vigente (lo renueva si le quedan menos de 30 s).
export async function accessToken(): Promise<string | null> {
  if (!keycloak.authenticated) return null;
  try {
    await keycloak.updateToken(30);
  } catch {
    return null;
  }
  return keycloak.token ?? null;
}

export const login = () => keycloak.login({ locale: 'es' });
export const register = () => keycloak.register({ locale: 'es' });
export const logout = () => keycloak.logout({ redirectUri: `${window.location.origin}/` });
