package main

import (
	"database/sql"
	"fmt"
	"strings"
)

// Reflect creates a reflection node synthesizing insights and links it
// to recently activated memory nodes (the current context).
func Reflect(db *sql.DB, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("texto de reflexión vacío")
	}

	now := nowMs()
	id := makeID("reflection")

	label := text
	if len(label) > 80 {
		label = label[:80] + "…"
	}

	// Create reflection node
	_, err := db.Exec(
		"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, 'reflection', ?, ?, '{}', ?, ?)",
		id, "reflexión: "+label, text, now, now,
	)
	if err != nil {
		return fmt.Errorf("creando reflexión: %w", err)
	}

	// Link to currently activated nodes (the context this reflects on)
	targets := 0
	rows, err := db.Query(`
		SELECT a.node_id, a.intensity, n.label
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > 0.05
		ORDER BY a.intensity DESC
		LIMIT 12
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var nid, nlabel string
			var intensity float64
			if err := rows.Scan(&nid, &intensity, &nlabel); err != nil {
				continue
			}
			edgeType := "semantic"
			weight := intensity * 0.8
			if weight < 0.3 {
				weight = 0.3
			}
			if _, err := AddEdge(db, id, nid, edgeType, weight); err != nil {
				continue
			}
			targets++
		}
	}

	// Also try to link to the most recent episode
	var recentID string
	err = db.QueryRow("SELECT id FROM nodes WHERE type = 'episode' ORDER BY created_at DESC LIMIT 1").Scan(&recentID)
	if err == nil {
		// Check not already linked from activation query
		AddEdge(db, id, recentID, "temporal", 0.6)
	}

	fmt.Printf("🧠 Reflexión creada: %s\n", label)
	fmt.Printf("   Enlaces contextuales: %d\n", targets)
	return nil
}

// Journal creates a session journal entry — a reflection that summarizes
// the most recent N episodes and the current state of the graph.
func Journal(db *sql.DB) error {
	now := nowMs()
	id := makeID("reflection")

	// Gather recent episodes as context
	epRows, err := db.Query(`
		SELECT id, label, content FROM nodes 
		WHERE type = 'episode' 
		ORDER BY created_at DESC 
		LIMIT 8
	`)
	if err != nil {
		return fmt.Errorf("consultando episodios recientes: %w", err)
	}
	defer epRows.Close()

	var recentEpisodes []struct {
		ID      string
		Label   string
		Content string
	}
	for epRows.Next() {
		var e struct {
			ID      string
			Label   string
			Content string
		}
		if err := epRows.Scan(&e.ID, &e.Label, &e.Content); err != nil {
			continue
		}
		recentEpisodes = append(recentEpisodes, e)
	}

	// Build journal content
	var journalParts []string
	journalParts = append(journalParts, "=== DIARIO DE SESIÓN ===")

	// Get stats
	var totalNodes, totalEdges, totalConsolidations int
	db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&totalNodes)
	db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&totalEdges)
	db.QueryRow("SELECT COALESCE(CAST(value AS INTEGER), 0) FROM stats WHERE key = 'total_consolidations'").Scan(&totalConsolidations)

	journalParts = append(journalParts, fmt.Sprintf("Grafo: %d nodos, %d aristas, %d consolidaciones", totalNodes, totalEdges, totalConsolidations))

	if len(recentEpisodes) > 0 {
		journalParts = append(journalParts, fmt.Sprintf("\nEpisodios recientes (%d):", len(recentEpisodes)))
		for i, ep := range recentEpisodes {
			summary := ep.Content
			if len(summary) > 120 {
				summary = summary[:120] + "…"
			}
			journalParts = append(journalParts, fmt.Sprintf("  %d. %s", i+1, summary))
		}
	}

	// Get hot concepts
	hotRows, err := db.Query(`
		SELECT n.label, a.intensity
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > 0.05
		ORDER BY a.intensity DESC
		LIMIT 6
	`)
	if err == nil {
		defer hotRows.Close()
		var hotConcepts []string
		for hotRows.Next() {
			var label string
			var intensity float64
			if err := hotRows.Scan(&label, &intensity); err == nil {
				hotConcepts = append(hotConcepts, fmt.Sprintf("%s (%.2f)", label, intensity))
			}
		}
		if len(hotConcepts) > 0 {
			journalParts = append(journalParts, "\nConceptos activos:")
			journalParts = append(journalParts, "  "+strings.Join(hotConcepts, ", "))
		}
	}

	journalText := strings.Join(journalParts, "\n")

	label := fmt.Sprintf("📓 Diario de sesión — %d episodios", len(recentEpisodes))

	// Create node
	_, err = db.Exec(
		"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, 'reflection', ?, ?, '{}', ?, ?)",
		id, label, journalText, now, now,
	)
	if err != nil {
		return fmt.Errorf("creando diario de sesión: %w", err)
	}

	// Link to recent episodes
	for _, ep := range recentEpisodes {
		AddEdge(db, id, ep.ID, "temporal", 0.7)
	}

	// Link to hot concepts
	if hotRows != nil {
		hotRows2, err := db.Query(`
			SELECT n.id FROM activation a JOIN nodes n ON a.node_id = n.id
			WHERE a.intensity > 0.05 ORDER BY a.intensity DESC LIMIT 6
		`)
		if err == nil {
			defer hotRows2.Close()
			for hotRows2.Next() {
				var nid string
				if err := hotRows2.Scan(&nid); err == nil {
					AddEdge(db, id, nid, "association", 0.5)
				}
			}
		}
	}

	fmt.Printf("📓 Diario de sesión guardado: %s\n", label)
	fmt.Printf("   Episodios enlazados: %d\n", len(recentEpisodes))
	
	// PWS: Consolidate session preferences
	ConsolidateSessionPreferences(db)
	fmt.Println("   📊 Preferencias de sesión consolidadas")

	return nil
}
