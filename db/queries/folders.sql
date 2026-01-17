-- name: GetFolder :one
SELECT * FROM folders WHERE id = ?;

-- name: GetFolderByPath :one
SELECT * FROM folders WHERE path = ?;

-- name: ListFolders :many
SELECT * FROM folders ORDER BY path;

-- name: ListSubfolders :many
SELECT * FROM folders WHERE path LIKE ? || '/%' AND path NOT LIKE ? || '/%/%' ORDER BY name;

-- name: CreateFolder :exec
INSERT INTO folders (id, path, name, created_at) VALUES (?, ?, ?, ?);

-- name: DeleteFolder :exec
DELETE FROM folders WHERE id = ?;

-- name: DeleteFolderByPath :exec
DELETE FROM folders WHERE path = ? OR path LIKE ? || '/%';
