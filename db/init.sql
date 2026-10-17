-- AgroMarket — una instancia de PostgreSQL con un esquema por microservicio.
-- Cada servicio solo lee/escribe su propio esquema (data ownership).

CREATE SCHEMA IF NOT EXISTS users;      -- User Service
CREATE SCHEMA IF NOT EXISTS catalog;    -- Catalog Service
CREATE SCHEMA IF NOT EXISTS orders;     -- Order Service
CREATE SCHEMA IF NOT EXISTS payments;   -- Payment Service
CREATE SCHEMA IF NOT EXISTS inventory;  -- Inventory Service

-- ---------- User Service ----------
CREATE TABLE users.accounts (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Catalog Service ----------
CREATE TABLE catalog.products (
  id          INT PRIMARY KEY,
  name        TEXT NOT NULL,
  variety     TEXT NOT NULL,
  origin      TEXT NOT NULL,
  unit        TEXT NOT NULL,
  price       NUMERIC(10,2) NOT NULL,
  stock       INT NOT NULL,
  image       TEXT NOT NULL,
  description TEXT NOT NULL
);

INSERT INTO catalog.products (id, name, variety, origin, unit, price, stock, image, description) VALUES
 (1, 'Palta Hass',        'Hass',            'La Libertad', 'caja 10 kg', 95.00,  120, '🥑', 'Calibre 16-18, materia seca > 23 %. Lista para exportación.'),
 (2, 'Arándano',          'Biloxi',          'Trujillo',    'caja 1.5 kg', 38.50, 200, '🫐', 'Clamshells de 125 g. Cadena de frío certificada.'),
 (3, 'Uva de mesa',       'Red Globe',       'Ica',         'caja 8.2 kg', 72.00,  90, '🍇', 'Bayas de 24 mm, color rojo intenso.'),
 (4, 'Mango',             'Kent',            'Piura',       'caja 4 kg',   34.90, 150, '🥭', 'Tratamiento hidrotérmico para mercado EE.UU.'),
 (5, 'Espárrago verde',   'UC-157',          'Ica',         'caja 5 kg',   58.00,  80, '🌱', 'Turiones de 22 cm, calibre standard.'),
 (6, 'Mandarina',         'W. Murcott',      'Lima',        'caja 10 kg',  45.00, 110, '🍊', 'Sin semilla, alto brix.'),
 (7, 'Café especial',     'Geisha',          'Cajamarca',   'saco 5 kg',  320.00,  25, '☕', 'Puntaje SCA 86+. Lavado, secado en camas africanas.'),
 (8, 'Quinua orgánica',   'Blanca Junín',    'Puno',        'saco 25 kg', 210.00,  40, '🌾', 'Certificación orgánica USDA / UE.'),
 (9, 'Cacao fino',        'Chuncho',         'Cusco',       'saco 10 kg', 280.00,  30, '🍫', 'Grano fermentado, cacao fino de aroma.');

-- ---------- Order Service ----------
CREATE TABLE orders.orders (
  id         SERIAL PRIMARY KEY,
  user_id    INT NOT NULL,
  status     TEXT NOT NULL,
  total      NUMERIC(12,2) NOT NULL,
  items      JSONB NOT NULL,
  timeline   JSONB NOT NULL DEFAULT '[]'::jsonb,
  reason     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON orders.orders (user_id);

-- ---------- Payment Service ----------
CREATE TABLE payments.payments (
  id         SERIAL PRIMARY KEY,
  order_id   INT NOT NULL,
  user_id    INT NOT NULL,
  amount     NUMERIC(12,2) NOT NULL,
  status     TEXT NOT NULL,
  reference  TEXT,
  reason     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Inventory Service ----------
CREATE TABLE inventory.stock (
  product_id INT PRIMARY KEY,
  quantity   INT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO inventory.stock (product_id, quantity)
SELECT id, stock FROM catalog.products;
