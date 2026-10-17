import { useCallback, useEffect, useMemo, useState } from 'react';
import { Analytics, api, ApiError, money, Notification, Order, Product, session, User } from './api';
import AuthModal from './components/AuthModal';
import OrderCard from './components/OrderCard';
import ArchitectureView from './components/ArchitectureView';

type Tab = 'catalogo' | 'pedidos' | 'arquitectura';

function usePolling(fn: () => void, ms: number, enabled = true) {
  useEffect(() => {
    if (!enabled) return;
    fn();
    const id = setInterval(fn, ms);
    return () => clearInterval(id);
  }, [fn, ms, enabled]);
}

export default function App() {
  const [user, setUser] = useState<User | null>(session.user);
  const [tab, setTab] = useState<Tab>('catalogo');
  const [products, setProducts] = useState<Product[]>([]);
  const [cart, setCart] = useState<Record<number, number>>({});
  const [orders, setOrders] = useState<Order[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [showAuth, setShowAuth] = useState(false);
  const [showCart, setShowCart] = useState(false);
  const [toast, setToast] = useState<{ text: string; kind: string } | null>(null);
  const [placing, setPlacing] = useState(false);
  const [offline, setOffline] = useState(!navigator.onLine);

  const notify = (text: string, kind = 'info') => {
    setToast({ text, kind });
    setTimeout(() => setToast(null), 3500);
  };

  useEffect(() => {
    const on = () => setOffline(false);
    const off = () => setOffline(true);
    window.addEventListener('online', on);
    window.addEventListener('offline', off);
    return () => { window.removeEventListener('online', on); window.removeEventListener('offline', off); };
  }, []);

  const handleAuthError = (e: unknown) => {
    if (e instanceof ApiError && e.status === 401) { session.clear(); setUser(null); }
  };

  const loadProducts = useCallback(() => { api.products().then(setProducts).catch(() => undefined); }, []);
  const loadUserData = useCallback(() => {
    api.orders().then(setOrders).catch(handleAuthError);
    api.notifications().then(setNotifications).catch(handleAuthError);
  }, []);
  const loadAnalytics = useCallback(() => { api.analytics().then(setAnalytics).catch(() => undefined); }, []);

  usePolling(loadProducts, 4000);
  usePolling(loadUserData, 1500, !!user);
  usePolling(loadAnalytics, 2000);

  const cartLines = useMemo(
    () => Object.entries(cart)
      .map(([id, qty]) => ({ product: products.find((p) => p.id === Number(id))!, qty }))
      .filter((l) => l.product),
    [cart, products],
  );
  const cartCount = cartLines.reduce((s, l) => s + l.qty, 0);
  const cartTotal = cartLines.reduce((s, l) => s + l.qty * l.product.price, 0);

  const add = (p: Product, delta = 1) =>
    setCart((c) => {
      const q = Math.max(0, Math.min(p.stock, (c[p.id] || 0) + delta));
      const next = { ...c, [p.id]: q };
      if (q === 0) delete next[p.id];
      return next;
    });

  const checkout = async () => {
    if (!user) { setShowAuth(true); return; }
    setPlacing(true);
    try {
      const res = await api.createOrder(cartLines.map((l) => ({ product_id: l.product.id, quantity: l.qty })));
      setCart({});
      setShowCart(false);
      setTab('pedidos');
      notify(`Pedido #${res.id} enviado al Event Bus (OrderCreated)`, 'success');
      loadUserData();
    } catch (e) {
      handleAuthError(e);
      notify((e as Error).message, 'error');
    } finally {
      setPlacing(false);
    }
  };

  const logout = () => { session.clear(); setUser(null); setOrders([]); setNotifications([]); };

  return (
    <div className="app">
      {offline && <div className="offline">Sin conexión — mostrando datos en caché (PWA)</div>}
      <header className="topbar">
        <div className="brand" onClick={() => setTab('catalogo')}>
          <img src="/icons/icon-192.png" alt="" />
          <div>
            <strong>AgroMarket</strong>
            <small>Agroexportación del Perú</small>
          </div>
        </div>
        <nav>
          {(['catalogo', 'pedidos', 'arquitectura'] as Tab[]).map((t) => (
            <button key={t} className={tab === t ? 'active' : ''} onClick={() => setTab(t)}>
              {t === 'catalogo' ? 'Catálogo' : t === 'pedidos' ? 'Mis pedidos' : 'Arquitectura'}
              {t === 'pedidos' && user && orders.length > 0 && <span className="pill">{orders.length}</span>}
            </button>
          ))}
        </nav>
        <div className="actions">
          <button className="btn ghost cart-btn" onClick={() => setShowCart(true)}>
            Carrito {cartCount > 0 && <span className="pill">{cartCount}</span>}
          </button>
          {user ? (
            <div className="user">
              <span>{user.name}</span>
              <button className="link" onClick={logout}>Salir</button>
            </div>
          ) : (
            <button className="btn primary" onClick={() => setShowAuth(true)}>Ingresar</button>
          )}
        </div>
      </header>

      <main>
        {tab === 'catalogo' && (
          <>
            <section className="hero">
              <div>
                <span className="eyebrow">Temporada 2026</span>
                <h1>Lo mejor del agro peruano, listo para exportar.</h1>
                <p>Palta de La Libertad, arándano de Trujillo, uva de Ica y café de Cajamarca. Arma tu pedido por lote y
                  síguelo en tiempo real mientras pasa por pago, inventario y despacho.</p>
              </div>
              <div className="hero-stats">
                <div><strong>{products.length}</strong><span>productos</span></div>
                <div><strong>{products.reduce((s, p) => s + p.stock, 0)}</strong><span>lotes en stock</span></div>
                <div><strong>{analytics?.orders_created ?? 0}</strong><span>pedidos hoy</span></div>
              </div>
            </section>

            <section className="grid">
              {products.map((p) => (
                <article key={p.id} className="card">
                  <div className="card-img">{p.image}</div>
                  <div className="card-body">
                    <div className="card-head">
                      <h3>{p.name}</h3>
                      <span className="variety">{p.variety}</span>
                    </div>
                    <p className="muted">{p.description}</p>
                    <div className="meta">
                      <span>📍 {p.origin}</span>
                      <span className={p.stock < 30 ? 'low' : ''}>{p.stock} en stock</span>
                    </div>
                    <div className="card-foot">
                      <div>
                        <strong>{money(p.price)}</strong>
                        <small> / {p.unit}</small>
                      </div>
                      {cart[p.id] ? (
                        <div className="qty">
                          <button onClick={() => add(p, -1)}>−</button>
                          <span>{cart[p.id]}</span>
                          <button onClick={() => add(p, 1)}>+</button>
                        </div>
                      ) : (
                        <button className="btn primary sm" disabled={p.stock === 0} onClick={() => add(p)}>Agregar</button>
                      )}
                    </div>
                  </div>
                </article>
              ))}
              {products.length === 0 && <p className="muted">Cargando catálogo desde Catalog Service…</p>}
            </section>
          </>
        )}

        {tab === 'pedidos' && (
          <section className="orders">
            <h1>Mis pedidos</h1>
            {!user && (
              <div className="empty">
                <p>Inicia sesión para ver tus pedidos.</p>
                <button className="btn primary" onClick={() => setShowAuth(true)}>Ingresar</button>
              </div>
            )}
            {user && orders.length === 0 && (
              <div className="empty">
                <p>Todavía no tienes pedidos.</p>
                <button className="btn primary" onClick={() => setTab('catalogo')}>Ir al catálogo</button>
              </div>
            )}
            <div className="orders-list">
              {orders.map((o) => <OrderCard key={o.id} order={o} />)}
            </div>
          </section>
        )}

        {tab === 'arquitectura' && <ArchitectureView analytics={analytics} notifications={notifications} />}
      </main>

      {showCart && (
        <div className="overlay" onClick={() => setShowCart(false)}>
          <aside className="drawer" onClick={(e) => e.stopPropagation()}>
            <header>
              <h2>Tu pedido</h2>
              <button className="link" onClick={() => setShowCart(false)}>Cerrar</button>
            </header>
            {cartLines.length === 0 && <p className="muted">El carrito está vacío.</p>}
            <ul className="cart-lines">
              {cartLines.map(({ product: p, qty }) => (
                <li key={p.id}>
                  <span className="emoji">{p.image}</span>
                  <div>
                    <strong>{p.name}</strong>
                    <small>{money(p.price)} / {p.unit}</small>
                  </div>
                  <div className="qty">
                    <button onClick={() => add(p, -1)}>−</button>
                    <span>{qty}</span>
                    <button onClick={() => add(p, 1)}>+</button>
                  </div>
                </li>
              ))}
            </ul>
            {cartLines.length > 0 && (
              <footer>
                <div className="total-row"><span>Total</span><strong>{money(cartTotal)}</strong></div>
                {cartTotal > 5000 && <p className="warn">Supera el límite de crédito de S/ 5,000: el Payment Service lo rechazará.</p>}
                <button className="btn primary block" disabled={placing} onClick={checkout}>
                  {placing ? 'Enviando…' : user ? 'Confirmar pedido' : 'Ingresa para comprar'}
                </button>
              </footer>
            )}
          </aside>
        </div>
      )}

      {showAuth && (
        <AuthModal
          onClose={() => setShowAuth(false)}
          onLogged={(u) => { setUser(u); setShowAuth(false); notify(`Bienvenido, ${u.name}`, 'success'); }}
        />
      )}

      {toast && <div className={`toast ${toast.kind}`}>{toast.text}</div>}

      <footer className="foot">
        AgroMarket · React + Webpack 5 → NGINX → Go → NATS → Go consumers → PostgreSQL + Redis
      </footer>
    </div>
  );
}
