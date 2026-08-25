-- PostgreSQL-native DOMAIN types (codegen emits generated/postgres/domains_gen.go)
CREATE DOMAIN user_email AS VARCHAR(255) CHECK (VALUE ~* '^.+@.+\..+$');
CREATE DOMAIN positive_int AS INTEGER NOT NULL CHECK (VALUE > 0);

CREATE TABLE accounts (
    id          BIGSERIAL PRIMARY KEY,
    email       user_email NOT NULL,
    score       positive_int,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
