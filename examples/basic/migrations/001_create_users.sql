-- 001_create_users.sql
CREATE TABLE IF NOT EXISTS users (
    id          VARCHAR(26) PRIMARY KEY,
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 002_create_orders.sql
CREATE TABLE IF NOT EXISTS orders (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(26) NOT NULL REFERENCES users(id),
    total       NUMERIC(12, 2) NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'Pending',
    notes       TEXT,
    placed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE orders ADD COLUMN updated_at TIMESTAMP;
