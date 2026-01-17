-- name: GetScript :one
SELECT * FROM scripts WHERE id = ?;

-- name: GetScriptByPath :one
SELECT * FROM scripts WHERE path = ?;

-- name: ListScripts :many
SELECT * FROM scripts ORDER BY path;

-- name: ListScriptsByFolder :many
SELECT * FROM scripts WHERE path LIKE ? || '/%' AND path NOT LIKE ? || '/%/%' ORDER BY name;

-- name: SearchScripts :many
SELECT * FROM scripts 
WHERE name LIKE '%' || ? || '%' 
   OR path LIKE '%' || ? || '%'
   OR description LIKE '%' || ? || '%'
   OR tags LIKE '%' || ? || '%'
ORDER BY path;

-- name: CreateScript :exec
INSERT INTO scripts (id, path, name, content, description, tags, locked, password_hash, danger_level, requires, examples, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateScript :exec
UPDATE scripts SET 
    path = ?,
    name = ?,
    content = ?,
    description = ?,
    tags = ?,
    locked = ?,
    password_hash = ?,
    danger_level = ?,
    requires = ?,
    examples = ?,
    updated_at = ?
WHERE id = ?;

-- name: UpdateScriptContent :exec
UPDATE scripts SET content = ?, updated_at = ? WHERE id = ?;

-- name: UpdateScriptLock :exec
UPDATE scripts SET locked = ?, password_hash = ?, updated_at = ? WHERE id = ?;

-- name: DeleteScript :exec
DELETE FROM scripts WHERE id = ?;

-- name: SetFavorite :exec
UPDATE scripts SET favorite = ? WHERE id = ?;

-- name: ListFavorites :many
SELECT * FROM scripts WHERE favorite = 1 ORDER BY path;

-- name: ListRecentlyUpdated :many
SELECT * FROM scripts ORDER BY updated_at DESC LIMIT ?;
