import { createRoot } from 'react-dom/client';
import App from './App';
import { initAuth } from './auth';
import './styles.css';

// 1) Recuperar la sesión de Keycloak (si existe) y 2) montar la app.
initAuth().then((user) => {
  createRoot(document.getElementById('root')!).render(<App initialUser={user} />);
});

// PWA: registrar el Service Worker generado por Workbox (solo en build de producción)
if ('serviceWorker' in navigator && process.env.NODE_ENV === 'production') {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js').catch(() => undefined);
  });
}
