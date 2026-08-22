-- name: CountActiveUsers :function
-- dbname: count_active_users
-- returns: int64
-- params: active
CREATE OR REPLACE FUNCTION count_active_users(p_active BOOLEAN)
RETURNS BIGINT AS $$
    SELECT COUNT(*)::BIGINT FROM users WHERE active = p_active
$$ LANGUAGE sql;

-- name: DeactivateUser :procedure
-- dbname: deactivate_user
-- params: email
CREATE OR REPLACE PROCEDURE deactivate_user(p_email VARCHAR(255))
LANGUAGE SQL AS $$
    UPDATE users SET active = false WHERE email = p_email;
$$;
