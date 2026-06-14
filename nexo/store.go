package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// reSentence finds complete sentences.
var reSentence = regexp.MustCompile(`[A-Z][^.!?]*[.!?]`)

// Store stores a text as an episode in the memory graph.
// Extracts entities, creates concept nodes, links them.
// importance (0.0–1.0) determines retention strength during consolidation.
func Store(db *sql.DB, text string, title string, importance float64) error {
	if title == "" {
		title = truncate("Nota: "+text, 60) + "..."
	}
	if importance < 0 {
		importance = 0
	} else if importance > 1.0 {
		importance = 1.0
	}

	// 1. Create episode node with importance in metadata
	metadata := fmt.Sprintf(`{"importance":%.2f}`, importance)
	now := nowMs()
	episodeID := makeID("episode")

	_, err := db.Exec(
		"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, 'episode', ?, ?, ?, ?, ?)",
		episodeID, title, text, metadata, now, now,
	)
	if err != nil {
		return fmt.Errorf("creating episode: %w", err)
	}

	// Insert into fulltext if content present
	if text != "" {
		db.Exec("INSERT INTO fulltext (node_id, content) VALUES (?, ?)", episodeID, text)
		db.Exec("INSERT INTO fulltext_fts (rowid, content) VALUES ((SELECT rowid FROM fulltext WHERE node_id = ?), ?)", episodeID, text)
	}

	// 2. Extract entities → concept nodes + edges
	conceptCount := 0
	entities := ExtractEntities(text)
	for _, ent := range entities {
		if strings.TrimSpace(ent.Value) == "" {
			continue
		}

		conceptID, err := AddNode(db, "concept", ent.Value, "", true)
		if err != nil {
			continue
		}

		edgeType := "semantic"
		if ent.Type == "phone" || ent.Type == "email" || ent.Type == "url" || ent.Type == "zip" {
			edgeType = "entity"
		}

		if _, err := AddEdge(db, episodeID, conceptID, edgeType, ent.Conf); err != nil {
			continue
		}
		conceptCount++
	}

	// 3. Extract facts (significant sentences)
	factCount := 0
	sentences := reSentence.FindAllString(text, -1)
	for _, sentence := range sentences {
		if len(sentence) < 30 {
			continue
		}
		if factCount >= 5 {
			break
		}

		factLabel := truncate(sentence, 80) + "..."
		factID, err := AddNode(db, "fact", factLabel, sentence, false)
		if err != nil {
			continue
		}
		if _, err := AddEdge(db, episodeID, factID, "semantic", 0.7); err != nil {
			continue
		}
		factCount++
	}

	fmt.Printf("✅ Almacenado en memoria:\n")
	fmt.Printf("   📋 Episodio: %s\n", truncate(episodeID, 28))
	fmt.Printf("   🏷️  Conceptos: %d\n", conceptCount)
	fmt.Printf("   💡 Hechos: %d\n", factCount)
	fmt.Printf("   ⭐ Importancia: %.2f\n", importance)

	// PWS: Analyze user pattern and update style vector
	AnalyzePattern(db, text, PatternTypeStore)
	UpdateStyleVector(db, text)

	return nil
}
