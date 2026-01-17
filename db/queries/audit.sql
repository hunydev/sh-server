-- name: CreateAuditLog :exec
INSERT INTO audit_log (action, entity_type, entity_id, entity_path, details, ip_address, user_agent, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAuditLogs :many
SELECT * FROM audit_log ORDER BY created_at DESC LIMIT ?;

-- name: ListAuditLogsByEntity :many
SELECT * FROM audit_log WHERE entity_id = ? ORDER BY created_at DESC;
