package graph

// SQLite schema DDL constants

const schemaNodes = `
CREATE TABLE IF NOT EXISTS nodes (
    rowid INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id TEXT UNIQUE NOT NULL,
    id TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_current INTEGER NOT NULL DEFAULT 1,
    type TEXT NOT NULL,
    content BLOB,
    properties TEXT,
    created_at DATETIME NOT NULL,
    modified_at DATETIME NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0,
    deleted_at DATETIME,
    change_note TEXT,
    changed_by TEXT,
    degree INTEGER NOT NULL DEFAULT 0
)`

const schemaLinks = `
CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    type TEXT NOT NULL,
    properties TEXT,
    created_at DATETIME NOT NULL,
    modified_at DATETIME NOT NULL,
    UNIQUE(source_id, target_id, type)
)`

const schemaVersionChain = `
CREATE TABLE IF NOT EXISTS version_chain (
    newer_version_id TEXT NOT NULL,
    older_version_id TEXT NOT NULL,
    PRIMARY KEY (newer_version_id, older_version_id)
)`

// FTS5 virtual table for full-text search
const schemaNodesFTS = `
CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
    id,
    type,
    content,
    properties,
    content='nodes',
    content_rowid='rowid'
)`

// Triggers to keep FTS index in sync with nodes table
const triggerFTSInsert = `
CREATE TRIGGER IF NOT EXISTS nodes_fts_insert AFTER INSERT ON nodes BEGIN
    INSERT INTO nodes_fts(rowid, id, type, content, properties)
    VALUES (NEW.rowid, NEW.id, NEW.type, NEW.content, NEW.properties);
END`

const triggerFTSDelete = `
CREATE TRIGGER IF NOT EXISTS nodes_fts_delete AFTER DELETE ON nodes BEGIN
    INSERT INTO nodes_fts(nodes_fts, rowid, id, type, content, properties)
    VALUES ('delete', OLD.rowid, OLD.id, OLD.type, OLD.content, OLD.properties);
END`

const triggerFTSUpdate = `
CREATE TRIGGER IF NOT EXISTS nodes_fts_update AFTER UPDATE ON nodes BEGIN
    INSERT INTO nodes_fts(nodes_fts, rowid, id, type, content, properties)
    VALUES ('delete', OLD.rowid, OLD.id, OLD.type, OLD.content, OLD.properties);
    INSERT INTO nodes_fts(rowid, id, type, content, properties)
    VALUES (NEW.rowid, NEW.id, NEW.type, NEW.content, NEW.properties);
END`

// Index definitions
const indexNodesID = `CREATE INDEX IF NOT EXISTS idx_nodes_id ON nodes(id)`
const indexNodesType = `CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type)`
const indexNodesIsCurrent = `CREATE INDEX IF NOT EXISTS idx_nodes_is_current ON nodes(is_current)`
const indexNodesDeleted = `CREATE INDEX IF NOT EXISTS idx_nodes_deleted ON nodes(deleted)`
const indexNodesVersionID = `CREATE INDEX IF NOT EXISTS idx_nodes_version_id ON nodes(version_id)`
const indexLinksSource = `CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_id)`
const indexLinksTarget = `CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_id)`
const indexLinksType = `CREATE INDEX IF NOT EXISTS idx_links_type ON links(type)`

// SQLite pragmas for optimal performance
const pragmaWAL = `PRAGMA journal_mode=WAL`
const pragmaFK = `PRAGMA foreign_keys=ON`
const pragmaBusyTimeout = `PRAGMA busy_timeout=5000`
const pragmaSynchronous = `PRAGMA synchronous=NORMAL`

// allSchemaStatements returns all schema DDL in order
func allSchemaStatements() []string {
	return []string{
		schemaNodes,
		schemaLinks,
		schemaVersionChain,
		schemaNodesFTS,
		triggerFTSInsert,
		triggerFTSDelete,
		triggerFTSUpdate,
		indexNodesID,
		indexNodesType,
		indexNodesIsCurrent,
		indexNodesDeleted,
		indexNodesVersionID,
		indexLinksSource,
		indexLinksTarget,
		indexLinksType,
	}
}

// allPragmas returns all pragma statements
func allPragmas() []string {
	return []string{
		pragmaWAL,
		pragmaFK,
		pragmaBusyTimeout,
		pragmaSynchronous,
	}
}
