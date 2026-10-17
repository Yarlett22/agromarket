import { Order, money } from '../api';

const STEPS = [
  { status: 'PENDIENTE', label: 'Pedido creado', event: 'OrderCreated', service: 'Order Service' },
  { status: 'PAGADO', label: 'Pago aprobado', event: 'PaymentCompleted', service: 'Payment Service' },
  { status: 'CONFIRMADO', label: 'Stock reservado', event: 'StockUpdated', service: 'Inventory Service' },
];

const time = (iso: string) => new Date(iso).toLocaleTimeString('es-PE', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

export default function OrderCard({ order }: { order: Order }) {
  const reached = (status: string) => order.timeline.find((t) => t.status === status);
  const rejected = order.status === 'RECHAZADO';

  return (
    <article className={`order ${order.status.toLowerCase()}`}>
      <header>
        <div>
          <span className="order-id">Pedido #{order.id}</span>
          <span className="muted"> · {new Date(order.created_at).toLocaleString('es-PE')}</span>
        </div>
        <span className={`badge ${order.status.toLowerCase()}`}>{order.status}</span>
      </header>

      <ol className="stepper">
        {STEPS.map((s, i) => {
          const r = reached(s.status);
          const failedHere = rejected && i === 1;
          const pending = !r && !failedHere && !rejected;
          const prevDone = i === 0 || reached(STEPS[i - 1].status);
          return (
            <li key={s.status} className={r ? 'done' : failedHere ? 'failed' : pending && prevDone ? 'active' : 'idle'}>
              <span className="dot">{r ? '✓' : failedHere ? '✕' : i + 1}</span>
              <div>
                <strong>{failedHere ? 'Pago rechazado' : s.label}</strong>
                <small>{failedHere ? 'PaymentFailed' : s.event} · {s.service}</small>
                <small>{r ? time(r.at) : failedHere ? time(reached('RECHAZADO')!.at) : pending && prevDone ? 'procesando…' : '—'}</small>
              </div>
            </li>
          );
        })}
      </ol>

      {order.reason && <div className="error">{order.reason}</div>}

      <ul className="order-items">
        {order.items.map((it) => (
          <li key={it.product_id}>
            <span>{it.quantity} × {it.name}</span>
            <span>{money(it.unit_price * it.quantity)}</span>
          </li>
        ))}
        <li className="total"><span>Total</span><span>{money(order.total)}</span></li>
      </ul>
    </article>
  );
}
