package main

import (
	"database/sql"
	"fmt"
	"strings"
)

// SpreadResult holds the result of activation propagation.
type SpreadResult struct {
	Seeds int
	Total int
}

// SpreadActivation runs the activation propagation algorithm.
// 1. Find seed nodes matching the query (by entity + FTS)
// 2. Activate seeds
// 3. Propagate via recursive CTE through edges
func SpreadActivation(db *sql.DB, cfg *DbConfig, query string) (*SpreadResult, error) {
	// 1. Reset previous activation
	if _, err := db.Exec("UPDATE activation SET intensity = 0"); err != nil {
		return nil, fmt.Errorf("resetting activation: %w", err)
	}

	// 2. Extract entities and find seed nodes
	seedMap := make(map[string]bool)
	entities := ExtractEntitiesQuery(query)

	for _, ent := range entities {
		rows, err := db.Query("SELECT id FROM nodes WHERE label LIKE ? LIMIT 5", "%"+ent.Value+"%")
		if err != nil {
			continue
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				seedMap[id] = true
			}
		}
		rows.Close()
	}

	// 3. FTS-based seeds (best-effort)
	ftsQuery := strings.ReplaceAll(query, "'", "''")
	rows, err := db.Query(`
		SELECT n.id FROM fulltext_fts fts 
		JOIN fulltext ft ON fts.rowid = ft.rowid 
		JOIN nodes n ON ft.node_id = n.id 
		WHERE fulltext_fts MATCH ? 
		LIMIT 10
	`, ftsQuery)
	if err == nil {
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				seedMap[id] = true
			}
		}
		rows.Close()
	}

	// 4. Activate seeds
	now := nowMs()
	seedCount := 0
	for id := range seedMap {
		_, err := db.Exec(`
			INSERT INTO activation (node_id, intensity, base_intensity, decay_rate, last_activated, activation_count) 
			VALUES (?, 1.0, 0.0, 0.1, ?, 1)
			ON CONFLICT(node_id) DO UPDATE SET 
				intensity = MAX(intensity, 1.0),
				last_activated = ?,
				activation_count = activation_count + 1
		`, id, now, now)
		if err == nil {
			seedCount++
		}
	}

	if seedCount == 0 {
		return &SpreadResult{0, 0}, nil
	}

	// 5. PROPAGATION: Recursive CTE
	propSQL := fmt.Sprintf(`
		WITH RECURSIVE propagation(node_id, intensity, hop) AS (
			SELECT a.node_id, a.intensity, 0
			FROM activation a
			WHERE a.intensity > 0.01

			UNION ALL

			SELECT 
				CASE WHEN e.source_id = p.node_id THEN e.target_id ELSE e.source_id END,
				p.intensity * %.2f * e.weight * 
					CASE e.type 
						WHEN 'semantic' THEN 1.0
						WHEN 'association' THEN 0.9
						WHEN 'entity' THEN 0.85
						WHEN 'temporal' THEN 0.7
						WHEN 'causal' THEN 1.1
						ELSE 0.8
					END,
				p.hop + 1
			FROM propagation p
			JOIN edges e ON e.source_id = p.node_id OR e.target_id = p.node_id
			WHERE p.hop < %d
			  AND p.intensity * %.2f > %.2f
		)
		INSERT INTO activation (node_id, intensity, base_intensity, decay_rate, last_activated, activation_count)
		SELECT 
			node_id, 
			MAX(intensity) as intensity, 
			0.0, 
			0.1, 
			%d, 
			1
		FROM propagation
		WHERE intensity > %.2f
		GROUP BY node_id
		ON CONFLICT(node_id) DO UPDATE SET
			intensity = MAX(intensity, EXCLUDED.intensity),
			last_activated = %d,
			activation_count = activation_count + 1
	`, cfg.ActivationDecay, cfg.MaxHops, cfg.ActivationDecay, cfg.ActivationThreshold, now, cfg.ActivationThreshold, now)

	if _, err := db.Exec(propSQL); err != nil {
		return nil, fmt.Errorf("propagation CTE: %w", err)
	}

	// 6. Count total activated
	var total int
	db.QueryRow("SELECT COUNT(*) FROM activation WHERE intensity > ?", cfg.ActivationThreshold).Scan(&total)

	return &SpreadResult{Seeds: seedCount, Total: total}, nil
}
