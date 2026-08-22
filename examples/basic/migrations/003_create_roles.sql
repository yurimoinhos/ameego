-- 003_create_roles.sql
CREATE TABLE IF NOT EXISTS roles (
    id          VARCHAR(26) PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT TRUE
);

ALTER TABLE users ADD COLUMN role_id VARCHAR(26) REFERENCES roles(id);
