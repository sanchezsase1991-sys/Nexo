package main

import (
	"database/sql"
	"fmt"
)

// Stats prints the memory graph statistics.
func Stats(db *sql.DB) error {
	var totalNodes, totalEdges int
	db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&totalNodes)
	db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&totalEdges)

	var episodes, facts, concepts, reflections int
	db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='episode'").Scan(&episodes)
	db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='fact'").Scan(&facts)
	db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='concept'").Scan(&concepts)
	db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='reflection'").Scan(&reflections)

	var consolidations, activations string
	db.QueryRow("SELECT COALESCE(value, '0') FROM stats WHERE key = 'total_consolidations'").Scan(&consolidations)
	db.QueryRow("SELECT COALESCE(value, '0') FROM stats WHERE key = 'total_activations'").Scan(&activations)

	fmt.Println()
	fmt.Println("🧠 MEMORY CORTEX - Estado del Grafo")
	fmt.Println("==================================================")
	fmt.Printf("📊 Total nodos:  %d\n", totalNodes)
	fmt.Printf("📊 Total aristas: %d\n", totalEdges)
	fmt.Println()
	fmt.Println("📦 Por tipo:")
	fmt.Printf("  📋 episode:    %d\n", episodes)
	fmt.Printf("  💡 fact:       %d\n", facts)
	fmt.Printf("  🏷️  concept:   %d\n", concepts)
	fmt.Printf("  🧠 reflection: %d\n", reflections)
	fmt.Println()
	fmt.Printf("🔄 Consolidaciones: %s\n", consolidations)
	fmt.Printf("⚡ Activaciones totales: %s\n", activations)
	fmt.Println()

	// Top concepts
	fmt.Println("🏆 Top conceptos:")
	topRows, err := db.Query(`
		SELECT n.label, COUNT(e.id) as conns
		FROM nodes n
		JOIN edges e ON e.target_id = n.id OR e.source_id = n.id
		WHERE n.type = 'concept'
		GROUP BY n.id
		ORDER BY conns DESC
		LIMIT 5
	`)
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var label string
			var conns int
			if err := topRows.Scan(&label, &conns); err == nil {
				fmt.Printf("  • %s (%d conexiones)\n", label, conns)
			}
		}
	}

	fmt.Println()
	return nil
}
