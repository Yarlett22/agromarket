// Cliente HTTP: todas las peticiones pasan por el API Gateway (/api/*) con JWT.

export interface Product {
  id: number;
  name: string;
  variety: string;
  origin: string;
  unit: string;
  price: number;
  stock: number;
  image: string;
  description: string;
}

export interface OrderItem {
  product_id: number;
  name: string;
  quantity: number;
  unit_price: number;
}

export interface TimelineEntry {
  status: string;
  event: string;
  at: string;
}

export interface Order {
  id: number;
  status: 'PENDIENTE' | 'PAGADO' | 'CONFIRMADO' | 'RECHAZADO';
  total: number;
  items: OrderItem[];
  timeline: TimelineEntry[];
  reason?: string;
  created_at: string;
}

export interface Notification {
  title: string;
  message: string;
  kind: 'info' | 'success' | 'error';
  event: string;
  order_id: number;
  at: string;
}

export interface Analytics {
  orders_created: number;
  payments_completed: number;
  payments_failed: number;
  stock_updates: number;
  events_total: number;
  revenue: number;
  top_products: { name: string; units: number }[];
}

export interface User {
  id: number | string;
  name: string;
  email: string;
}

const TOKEN_KEY = 'agromarket.token';
const USER_KEY = 'agromarket.user';

export const session = {
  get token(): string | null {
    try { return localStorage.getItem(TOKEN_KEY); } catch { return null; }
  },
  get user(): User | null {
    try { const u = localStorage.getItem(USER_KEY); return u ? JSON.parse(u) : null; } catch { return null; }
  },
  save(token: string, user: User) {
    try { localStorage.setItem(TOKEN_KEY, token); localStorage.setItem(USER_KEY, JSON.stringify(user)); } catch { /* noop */ }
  },
  clear() {
    try { localStorage.removeItem(TOKEN_KEY); localStorage.removeItem(USER_KEY); } catch { /* noop */ }
  },
};

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = session.token;
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`/api${path}`, { ...init, headers: { ...headers, ...(init.headers as object) } });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 429) throw new ApiError(429, 'Demasiadas solicitudes: el API Gateway aplicó rate limiting.');
    throw new ApiError(res.status, body.error || `Error ${res.status}`);
  }
  return body as T;
}

export const api = {
  register: (name: string, email: string, password: string) =>
    request<{ token: string; user: User }>('/users/register', { method: 'POST', body: JSON.stringify({ name, email, password }) }),
  login: (email: string, password: string) =>
    request<{ token: string; user: User }>('/users/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  products: () => request<Product[]>('/catalog/products'),
  createOrder: (items: { product_id: number; quantity: number }[]) =>
    request<{ id: number; status: string; total: number }>('/orders', { method: 'POST', body: JSON.stringify({ items }) }),
  orders: () => request<Order[]>('/orders'),
  notifications: () => request<Notification[]>('/notifications'),
  analytics: () => request<Analytics>('/analytics'),
};

export const money = (n: number) =>
  `S/ ${n.toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
