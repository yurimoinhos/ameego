-- 004_create_active_users.sql
CREATE VIEW active_users AS
SELECT id, email, name, role_id, created_at
FROM users
WHERE active = true;
