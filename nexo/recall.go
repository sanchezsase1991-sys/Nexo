package main

import (
	"database/sql"
	"fmt"
	"strings"
)

// ActivatedNode represents a recalled memory node.
type ActivatedNode struct {
	Icon      string
	Type      string
	Label     string
	Intensity float64
	Content   string
}

// Recall retrieves the full associative context for a query.
func Recall(db *sql.DB, cfg *DbConfig, query string) error {
	entities := ExtractEntities(query)

	result, err := SpreadActivation(db, cfg, query)
	if err != nil {
		return fmt.Errorf("spreading activation: %w", err)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     MEMORIA ASOCIATIVA RECUPERADA       ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	shortQ := query
	if len(shortQ) > 80 {
		shortQ = shortQ[:80]
	}
	fmt.Printf("🔍 Query: %s\n", shortQ)
	fmt.Printf("📊 Nodos activados: %d | Semilla: %d\n", result.Total, result.Seeds)
	fmt.Println()

	// Show detected entities
	if len(entities) > 0 {
		var labels []string
		for _, e := range entities {
			labels = append(labels, e.Value)
		}
		fmt.Printf("🏷️  Entidades detectadas: %s\n\n", strings.Join(labels, ", "))
	}

	// Get activated nodes
	nodes, err := getActivatedNodes(db, cfg)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		fmt.Printf("🧠 No se encontraron memorias asociadas a: %s\n", query)
	}

	for _, n := range nodes {
		// Build activation bar
		barLen := int(n.Intensity * 20)
		if barLen > 20 {
			barLen = 20
		}
		if barLen < 0 {
			barLen = 0
		}
		bar := strings.Repeat("█", barLen)

		fmt.Printf("  %s [%s] %s  %s %.2f\n", n.Icon, n.Type, n.Label, green(bar), n.Intensity)
		if n.Content != "" {
			shortContent := n.Content
			if len(shortContent) > 100 {
				shortContent = shortContent[:100]
			}
			fmt.Printf("     └─ %s\n", shortContent)
		}
	}

	// Stats
	var totalNodes, totalEdges int
	db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&totalNodes)
	db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&totalEdges)
	fmt.Println()
	fmt.Printf("📈 Stats: %d nodos, %d aristas en grafo\n", totalNodes, totalEdges)

	// Update stats
	db.Exec("UPDATE stats SET value = ? WHERE key = 'last_query'", query)
	db.Exec("UPDATE stats SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT) WHERE key = 'total_activations'")

	return nil
}

// RecallBrief returns a compact memory summary for agent integration.
func RecallBrief(db *sql.DB, cfg *DbConfig, query string) error {
	result, err := SpreadActivation(db, cfg, query)
	if err != nil {
		return err
	}

	if result.Total == 0 {
		return nil
	}

	fmt.Printf("🧠 [Memoria] Query: %s\n", truncate(query, 60))
	fmt.Printf("📊 Activados: %d nodos\n", result.Total)

	nodes, err := getActivatedBriefNodes(db, cfg)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		fmt.Printf("  %s %s [%.1f]\n", n.Icon, n.Label, n.Intensity)
	}

	return nil
}

func getActivatedNodes(db *sql.DB, cfg *DbConfig) ([]ActivatedNode, error) {
	rows, err := db.Query(`
		SELECT 
			CASE n.type 
				WHEN 'episode' THEN '📋'
				WHEN 'fact' THEN '💡'
				WHEN 'concept' THEN '🏷️'
				WHEN 'reflection' THEN '🧠'
				ELSE '📄'
			END,
			n.type,
			n.label,
			a.intensity,
			COALESCE(substr(n.content, 1, 120), '')
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > ?
		ORDER BY a.intensity DESC
		LIMIT ?
	`, cfg.ActivationThreshold, cfg.MaxContext)
	if err != nil {
		return nil, fmt.Errorf("querying activated nodes: %w", err)
	}
	defer rows.Close()

	var nodes []ActivatedNode
	for rows.Next() {
		var n ActivatedNode
		if err := rows.Scan(&n.Icon, &n.Type, &n.Label, &n.Intensity, &n.Content); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func getActivatedBriefNodes(db *sql.DB, cfg *DbConfig) ([]ActivatedNode, error) {
	rows, err := db.Query(`
		SELECT 
			CASE n.type 
				WHEN 'episode' THEN '📋'
				WHEN 'fact' THEN '💡'
				WHEN 'concept' THEN '🏷️'
				WHEN 'reflection' THEN '🧠'
				ELSE '•'
			END,
			n.label,
			a.intensity
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > ?
		ORDER BY a.intensity DESC
		LIMIT 8
	`, cfg.ActivationThreshold)
	if err != nil {
		return nil, fmt.Errorf("querying brief activated nodes: %w", err)
	}
	defer rows.Close()

	var nodes []ActivatedNode
	for rows.Next() {
		var n ActivatedNode
		if err := rows.Scan(&n.Icon, &n.Label, &n.Intensity); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func green(s string) string {
	return "\033[0;32m" + s + "\033[0m"
}
