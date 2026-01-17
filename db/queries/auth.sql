-- name: CreateAuthToken :exec
INSERT INTO auth_tokens (token, script_id, expires_at, created_at, ip_address, user_agent)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetAuthToken :one
SELECT * FROM auth_tokens WHERE token = ?;

-- name: DeleteExpiredTokens :exec
DELETE FROM auth_tokens WHERE expires_at < ?;

-- name: DeleteTokensByScript :exec
DELETE FROM auth_tokens WHERE script_id = ?;
