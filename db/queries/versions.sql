-- name: CreateVersion :exec
INSERT INTO script_versions (script_id, content, version, created_at)
VALUES (?, ?, ?, ?);

-- name: GetLatestVersion :one
SELECT MAX(version) as version FROM script_versions WHERE script_id = ?;

-- name: ListVersions :many
SELECT * FROM script_versions WHERE script_id = ? ORDER BY version DESC;

-- name: GetVersion :one
SELECT * FROM script_versions WHERE script_id = ? AND version = ?;
