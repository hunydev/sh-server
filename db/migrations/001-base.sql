-- Base schema for sh.huny.dev
--
-- Migrations tracking table
CREATE TABLE IF NOT EXISTS migrations (
    migration_number INTEGER PRIMARY KEY,
    migration_name TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Scripts table (main entity)
CREATE TABLE IF NOT EXISTS scripts (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,        -- e.g., /tools/check_memory.sh
    name TEXT NOT NULL,               -- e.g., check_memory.sh
    content TEXT NOT NULL DEFAULT '', -- actual script content
    description TEXT DEFAULT '',
    tags TEXT DEFAULT '',             -- comma-separated tags
    locked INTEGER NOT NULL DEFAULT 0,
    password_hash TEXT,               -- bcrypt hash if locked
    danger_level INTEGER DEFAULT 0,   -- 0=safe, 1=caution, 2=dangerous
    requires TEXT DEFAULT '',         -- comma-separated requirements (e.g., curl,jq)
    examples TEXT DEFAULT '',         -- usage examples
    favorite INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_scripts_path ON scripts(path);
CREATE INDEX IF NOT EXISTS idx_scripts_name ON scripts(name);

-- Folders table (virtual folders for organization)
CREATE TABLE IF NOT EXISTS folders (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,        -- e.g., /tools/monitoring
    name TEXT NOT NULL,               -- e.g., monitoring
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_folders_path ON folders(path);

-- Auth tokens for locked script access
CREATE TABLE IF NOT EXISTS auth_tokens (
    token TEXT PRIMARY KEY,
    script_id TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT,
    FOREIGN KEY (script_id) REFERENCES scripts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_script ON auth_tokens(script_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires ON auth_tokens(expires_at);

-- Audit log (optional but recommended)
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,             -- CREATE, UPDATE, DELETE, ACCESS, UNLOCK_ATTEMPT
    entity_type TEXT NOT NULL,        -- script, folder
    entity_id TEXT,
    entity_path TEXT,
    details TEXT,                     -- JSON details
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);

-- Script versions (simple versioning)
CREATE TABLE IF NOT EXISTS script_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    script_id TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (script_id) REFERENCES scripts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_versions_script ON script_versions(script_id);

-- Record execution of this migration
INSERT OR IGNORE INTO migrations (migration_number, migration_name)
VALUES (001, '001-base');
