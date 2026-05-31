package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DbConfig holds database configuration.
type DbConfig struct {
	Path         string
	ActivationDecay    float64
	ActivationThreshold float64
	MaxHops       int
	MaxContext    int
}

var DefaultConfig = DbConfig{
	Path:                getEnvOrDefault("MEMORY_CORTEX_DIR", "/root/.memory-cortex") + "/graph.db",
	ActivationDecay:     0.3,
	ActivationThreshold: 0.15,
	MaxHops:             4,
	MaxContext:          20,
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// OpenDB opens a connection to the SQLite database.
func OpenDB(cfg *DbConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode and foreign keys
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragmas: %w", err)
		}
	}

	return db, nil
}

// InitDB creates the full database schema.
func InitDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL CHECK(type IN ('episode','fact','concept','reflection')),
		label TEXT NOT NULL,
		content TEXT DEFAULT '',
		metadata TEXT DEFAULT '{}',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
	CREATE INDEX IF NOT EXISTS idx_nodes_label ON nodes(label);

	CREATE TABLE IF NOT EXISTS edges (
		id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL,
		target_id TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('temporal','semantic','causal','entity','association')),
		weight REAL DEFAULT 1.0,
		metadata TEXT DEFAULT '{}',
		created_at INTEGER NOT NULL,
		FOREIGN KEY (source_id) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (target_id) REFERENCES nodes(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);
	CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);
	CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);

	CREATE TABLE IF NOT EXISTS activation (
		node_id TEXT PRIMARY KEY,
		intensity REAL DEFAULT 0.0,
		base_intensity REAL DEFAULT 0.0,
		decay_rate REAL DEFAULT 0.1,
		last_activated INTEGER DEFAULT 0,
		activation_count INTEGER DEFAULT 0,
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS fulltext (
		node_id TEXT PRIMARY KEY,
		content TEXT DEFAULT '',
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS embeddings_cache (
		text_hash TEXT PRIMARY KEY,
		tokens TEXT DEFAULT '[]',
		entities TEXT DEFAULT '[]',
		created_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS stats (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	// Try to create FTS5 virtual table (may fail if not compiled with FTS5)
	ftsSQL := `CREATE VIRTUAL TABLE IF NOT EXISTS fulltext_fts USING fts5(content, content='fulltext', content_rowid='rowid')`
	if _, err := db.Exec(ftsSQL); err != nil {
		// FTS5 not available — log but continue
		fmt.Fprintf(os.Stderr, "⚠️  FTS5 no disponible (continuando sin búsqueda de texto completo): %v\n", err)
	}

	// Ensure default stats exist
	defaults := map[string]string{
		"version":             "2",
		"created_at":          fmt.Sprintf("%d", time.Now().Unix()),
		"total_activations":   "0",
		"total_consolidations": "0",
		"last_query":          "",
	}
	for k, v := range defaults {
		_, err := db.Exec("INSERT OR IGNORE INTO stats (key, value) VALUES (?, ?)", k, v)
		if err != nil {
			return fmt.Errorf("inserting stats: %w", err)
		}
	}

	return nil
}

// makeID generates a unique node/edge ID using a timestamp + crypto random suffix.
func makeID(prefix string) string {
	now := time.Now().UnixNano()
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	randPart := hex.EncodeToString(randBytes)
	return fmt.Sprintf("%s_%d_%s", prefix, now, randPart)
}

// nowMs returns current Unix timestamp in seconds.
func nowMs() int64 {
	return time.Now().Unix()
}

// AddNode creates a new node or returns an existing one with the same label+type.
func AddNode(db *sql.DB, ntype, label, content string, checkDupe bool) (string, error) {
	if checkDupe {
		var existing string
		err := db.QueryRow("SELECT id FROM nodes WHERE label = ? AND type = ? LIMIT 1", label, ntype).Scan(&existing)
		if err == nil {
			return existing, nil
		}
	}

	id := makeID(ntype)
	now := nowMs()

	_, err := db.Exec(
		"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, ?, ?, ?, '{}', ?, ?)",
		id, ntype, label, content, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("inserting node: %w", err)
	}

	// Insert into fulltext if content present
	if strings.TrimSpace(content) != "" {
		_, err = db.Exec("INSERT INTO fulltext (node_id, content) VALUES (?, ?)", id, content)
		if err != nil {
			return id, fmt.Errorf("inserting fulltext: %w", err)
		}
		// Try FTS insert (best-effort)
		_, _ = db.Exec("INSERT INTO fulltext_fts (rowid, content) VALUES ((SELECT rowid FROM fulltext WHERE node_id = ?), ?)", id, content)
	}

	return id, nil
}

// AddEdge creates a new edge with Hebbian reinforcement on duplicates.
func AddEdge(db *sql.DB, source, target, etype string, weight float64) (string, error) {
	// Check existing
	var existing string
	err := db.QueryRow(
		"SELECT id FROM edges WHERE source_id = ? AND target_id = ? AND type = ? LIMIT 1",
		source, target, etype,
	).Scan(&existing)
	if err == nil {
		// Hebbian reinforcement
		_, err := db.Exec(
			"UPDATE edges SET weight = MIN(weight + 0.05, 2.0), metadata = json_set(COALESCE(metadata, '{}'), '$.reinforced', COALESCE(json_extract(COALESCE(metadata, '{}'), '$.reinforced'), 0) + 1) WHERE id = ?",
			existing,
		)
		return existing, err
	}

	id := makeID("e")
	now := nowMs()
	_, err = db.Exec(
		"INSERT INTO edges (id, source_id, target_id, type, weight, metadata, created_at) VALUES (?, ?, ?, ?, ?, '{}', ?)",
		id, source, target, etype, weight, now,
	)
	if err != nil {
		return "", fmt.Errorf("inserting edge: %w", err)
	}
	return id, nil
}
