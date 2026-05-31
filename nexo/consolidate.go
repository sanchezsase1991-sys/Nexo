package main

import (
	"database/sql"
	"fmt"
)

// Consolidate runs the memory consolidation (sleep) process:
// 1. Hebbian learning — reinforce co-occurring concept connections
// 2. Reflections — create hub nodes for strongly connected concepts
// 3. Activation decay — simulate forgetting
// 4. Prune weak connections
// 5. Clean stale activations
func Consolidate(db *sql.DB) error {
	now := nowMs()

	fmt.Println("=== CONSOLIDACIÓN (SUEÑO) ===")

	// --- 1. Hebbian Learning ---
	fmt.Println("ℹ Reforzando conexiones Hebbian...")
	hebbianCount := 0

	rows, err := db.Query(`
		SELECT e1.target_id as concept_a, e2.target_id as concept_b, COUNT(*) as cooc
		FROM edges e1
		JOIN edges e2 ON e1.source_id = e2.source_id
		WHERE e1.type = 'semantic' AND e2.type = 'semantic'
		  AND e1.target_id < e2.target_id
		  AND e1.source_id != e2.source_id
		GROUP BY e1.target_id, e2.target_id
		HAVING cooc >= 2
		LIMIT 50
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ca, cb string
			var cooc int
			if err := rows.Scan(&ca, &cb, &cooc); err != nil {
				continue
			}

			// Check existing edge
			var existing string
			err := db.QueryRow(
				"SELECT id FROM edges WHERE source_id = ? AND target_id = ? AND type = 'association' LIMIT 1",
				ca, cb,
			).Scan(&existing)
			if err == nil {
				db.Exec("UPDATE edges SET weight = MIN(weight + 0.1, 2.0) WHERE id = ?", existing)
			} else {
				id := makeID("e")
				db.Exec(
					"INSERT INTO edges (id, source_id, target_id, type, weight, metadata, created_at) VALUES (?, ?, ?, 'association', 0.5, '{\"hebbian\":true}', ?)",
					id, ca, cb, now,
				)
			}
			hebbianCount++
		}
	}
	fmt.Printf("✓ Conexiones Hebbian reforzadas: %d\n", hebbianCount)

	// --- 2. Reflections ---
	fmt.Println("ℹ Creando reflexiones...")
	reflectionCount := 0

	refRows, err := db.Query(`
		SELECT n.id, n.label, COUNT(e.id) as conn_count
		FROM nodes n
		JOIN edges e ON e.target_id = n.id OR e.source_id = n.id
		WHERE n.type = 'concept'
		GROUP BY n.id
		HAVING conn_count >= 5
		ORDER BY conn_count DESC
		LIMIT 5
	`)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			var nid, nlabel string
			var conns int
			if err := refRows.Scan(&nid, &nlabel, &conns); err != nil {
				continue
			}

			// Check if reflection already exists
			var existing string
			err := db.QueryRow(
				"SELECT id FROM nodes WHERE label = ? AND type = 'reflection' LIMIT 1",
				"reflection: "+nlabel,
			).Scan(&existing)
			if err == nil {
				db.Exec("UPDATE nodes SET updated_at = ? WHERE id = ?", now, existing)
				continue
			}

			// Create reflection
			reflID := makeID("r")
			reflContent := fmt.Sprintf("Núcleo conceptual con %d conexiones. El concepto %s aparece recurrentemente en múltiples contextos.", conns, nlabel)
			db.Exec(
				"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, 'reflection', ?, ?, '{\"hub\":true}', ?, ?)",
				reflID, "reflection: "+nlabel, reflContent, now, now,
			)
			AddEdge(db, reflID, nid, "semantic", 1.0)
			reflectionCount++
		}
	}
	fmt.Printf("✓ Reflexiones creadas: %d\n", reflectionCount)

	// --- 3. Activation decay (importance-aware) ---
	fmt.Println("ℹ Aplicando decaimiento de activación (con conciencia de importancia)...")

	// High-importance nodes decay less; low-importance decay more
	db.Exec(`
		UPDATE activation 
		SET intensity = intensity * (
			CASE 
				WHEN CAST(json_extract(COALESCE(n.metadata, '{}'), '$.importance') AS REAL) > 0.7 THEN 0.85
				WHEN CAST(json_extract(COALESCE(n.metadata, '{}'), '$.importance') AS REAL) > 0.4 THEN 0.75
				WHEN CAST(json_extract(COALESCE(n.metadata, '{}'), '$.importance') AS REAL) > 0.0 THEN 0.60
				ELSE 0.50
			END
		)
		FROM nodes n
		WHERE activation.node_id = n.id
		  AND activation.intensity > 0
	`)
	// Also decay nodes without explicit importance (type-based default)
	db.Exec(`
		UPDATE activation 
		SET intensity = intensity * 0.65
		WHERE node_id NOT IN (
			SELECT id FROM nodes WHERE json_extract(COALESCE(metadata, '{}'), '$.importance') IS NOT NULL
		) AND intensity > 0
	`)
	fmt.Println("✓ Decaimiento selectivo aplicado")

	// --- 4. Prune weak connections (importance-aware) ---
	fmt.Println("ℹ Podando conexiones débiles...")
	// Don't prune edges connected to high-importance nodes, even if weight is low
	var pruned int
	db.QueryRow(`
		SELECT COUNT(*) FROM edges e
		WHERE e.weight < 0.1 
		  AND e.type = 'association'
		  AND e.source_id NOT IN (
			  SELECT id FROM nodes 
			  WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.6
		  )
		  AND e.target_id NOT IN (
			  SELECT id FROM nodes 
			  WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.6
		  )
	`).Scan(&pruned)
	db.Exec(`
		DELETE FROM edges WHERE id IN (
			SELECT e.id FROM edges e
			WHERE e.weight < 0.1 
			  AND e.type = 'association'
			  AND e.source_id NOT IN (
				  SELECT id FROM nodes 
				  WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.6
			  )
			  AND e.target_id NOT IN (
				  SELECT id FROM nodes 
				  WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.6
			  )
		)
	`)
	fmt.Printf("✓ Aristas podadas: %d\n", pruned)

	// --- 5. Strengthen high-importance connections ---
	fmt.Println("ℹ Reforzando nodos de alta importancia...")
	db.Exec(`
		UPDATE edges SET weight = MIN(weight + 0.05, 2.0)
		WHERE source_id IN (
			SELECT id FROM nodes 
			WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.7
		)
		OR target_id IN (
			SELECT id FROM nodes 
			WHERE CAST(json_extract(COALESCE(metadata, '{}'), '$.importance') AS REAL) > 0.7
		)
	`)
	fmt.Println("✓ Conexiones importantes reforzadas")

	// --- 6. Clean stale activations ---
	oldThreshold := now - 86400*7
	db.Exec("DELETE FROM activation WHERE last_activated < ? AND intensity < 0.1", oldThreshold)

	// --- 7. Update stats ---
	var count int
	db.QueryRow("SELECT COALESCE(CAST(value AS INTEGER), 0) FROM stats WHERE key = 'total_consolidations'").Scan(&count)
	count++
	db.Exec("UPDATE stats SET value = ? WHERE key = 'total_consolidations'", fmt.Sprintf("%d", count))

	fmt.Println()
	fmt.Printf("✅ Consolidación #%d completada (con importancia):\n", count)
	fmt.Printf("   • Hebbian: %d conexiones\n", hebbianCount)
	fmt.Printf("   • Reflexiones: %d\n", reflectionCount)
	fmt.Printf("   • Aristas podadas (no importantes): %d\n", pruned)
	fmt.Printf("   • Decaimiento: selectivo por importancia\n")

	return nil
}


