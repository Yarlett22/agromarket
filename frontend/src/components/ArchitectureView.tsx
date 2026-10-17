import { useEffect, useRef, useState } from 'react';
import { Analytics, Notification, money } from '../api';

interface Props {
  analytics: Analytics | null;
  notifications: Notification[];
}

const LAYERS = [
  { name: 'Frontend', tech: 'React 18 · TypeScript · Webpack 5 · PWA', nodes: ['React Web', 'Service Worker', 'Manifest'] },
  { name: 'API Gateway', tech: 'NGINX · JWT (auth_request) · Rate limiting · Logging', nodes: ['/api/users', '/api/catalog', '/api/orders', '/api/notifications'] },
  { name: 'Backend', tech: 'Go 1.23 · REST', nodes: ['User Service', 'Catalog Service', 'Order Service'] },
  { name: 'Event Bus', tech: 'NATS', nodes: ['orders.created', 'payments.completed', 'payments.failed', 'stock.updated'] },
  { name: 'Event Consumers', tech: 'Go', nodes: ['Payment Service', 'Inventory Service', 'Notification Service', 'Analytics Service'] },
  { name: 'Datos', tech: 'PostgreSQL (un esquema por servicio) · Redis', nodes: ['PostgreSQL', 'Redis cache', 'Redis notificaciones'] },
];

export default function ArchitectureView({ analytics, notifications }: Props) {
  const [pulse, setPulse] = useState(false);
  const last = useRef(0);

  useEffect(() => {
    const total = analytics?.events_total ?? 0;
    if (total > last.current) {
      setPulse(true);
      const t = setTimeout(() => setPulse(false), 900);
      last.current = total;
      return () => clearTimeout(t);
    }
    last.current = total;
  }, [analytics?.events_total]);

  const a = analytics;
  return (
    <section className="arch">
      <div className="arch-intro">
        <h1>Arquitectura en vivo</h1>
        <p className="muted">
          Frontend → API Gateway → Backend → Event Bus → Event Consumers → Datos. Haz un pedido y observa cómo los
          eventos atraviesan el sistema.
        </p>
      </div>

      <div className="kpis">
        <div className="kpi"><span>Pedidos creados</span><strong>{a?.orders_created ?? 0}</strong></div>
        <div className="kpi"><span>Pagos aprobados</span><strong>{a?.payments_completed ?? 0}</strong></div>
        <div className="kpi"><span>Pagos rechazados</span><strong>{a?.payments_failed ?? 0}</strong></div>
        <div className="kpi"><span>Eventos procesados</span><strong>{a?.events_total ?? 0}</strong></div>
        <div className="kpi wide"><span>Ingresos confirmados</span><strong>{money(a?.revenue ?? 0)}</strong></div>
      </div>

      <div className="arch-grid">
        <div className="layers">
          {LAYERS.map((l, i) => (
            <div key={l.name} className={`layer ${l.name === 'Event Bus' && pulse ? 'pulse' : ''}`}>
              <div className="layer-head">
                <span className="layer-n">{i + 1}</span>
                <div>
                  <strong>{l.name}</strong>
                  <small>{l.tech}</small>
                </div>
              </div>
              <div className="nodes">
                {l.nodes.map((n) => <span key={n} className="node">{n}</span>)}
              </div>
            </div>
          ))}
        </div>

        <aside className="events">
          <h3>Flujo de eventos (tus pedidos)</h3>
          {notifications.length === 0 && <p className="muted">Aún no hay eventos. Realiza un pedido desde el catálogo.</p>}
          <ul>
            {notifications.map((n, i) => (
              <li key={i} className={n.kind}>
                <code>{n.event}</code>
                <span>{n.message}</span>
                <time>{new Date(n.at).toLocaleTimeString('es-PE')}</time>
              </li>
            ))}
          </ul>
          {a && a.top_products.length > 0 && (
            <>
              <h3>Más vendidos (Analytics Service)</h3>
              <ul className="top">
                {a.top_products.map((p) => (
                  <li key={p.name}><span>{p.name}</span><b>{p.units} u.</b></li>
                ))}
              </ul>
            </>
          )}
        </aside>
      </div>
    </section>
  );
}
