package main

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// DeepRecall runs multi-hop recall showing the association chains.
// After standard propagation, traces each activated node back to seed nodes
// and displays the hop distance and connection path.
func DeepRecall(db *sql.DB, cfg *DbConfig, query string) error {
	// Run standard propagation first
	result, err := SpreadActivation(db, cfg, query)
	if err != nil {
		return fmt.Errorf("spreading activation: %w", err)
	}

	shortQ := query
	if len(shortQ) > 60 {
		shortQ = shortQ[:60]
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║     📡 RECALL PROFUNDO                      ║")
	fmt.Println("║     Cadenas de asociación multihop          ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("🔍 Query: %s\n", shortQ)
	fmt.Printf("📊 Nodos activados: %d | Semilla: %d\n", result.Total, result.Seeds)
	fmt.Println()

	if result.Total == 0 {
		fmt.Println("🧠 No se encontraron memorias asociadas.")
		return nil
	}

	// We need IDs to trace paths, so query with IDs directly.
	type FullNode struct {
		ID        string
		Icon      string
		Type      string
		Label     string
		Intensity float64
		Content   string
	}

	rows, err := db.Query(`
		SELECT n.id,
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
			COALESCE(substr(n.content, 1, 100), '')
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > ?
		ORDER BY a.intensity DESC
		LIMIT ?
	`, cfg.ActivationThreshold, cfg.MaxContext)
	if err != nil {
		return fmt.Errorf("querying deep nodes: %w", err)
	}
	defer rows.Close()

	var allNodes []FullNode
	for rows.Next() {
		var n FullNode
		if err := rows.Scan(&n.ID, &n.Icon, &n.Type, &n.Label, &n.Intensity, &n.Content); err != nil {
			continue
		}
		allNodes = append(allNodes, n)
	}

	if len(allNodes) == 0 {
		return nil
	}

	// Identify seed nodes (intensity >= 0.9, the directly matched ones)
	seedIDs := make(map[string]bool)
	for _, n := range allNodes {
		if n.Intensity >= 0.9 {
			seedIDs[n.ID] = true
		}
	}

	// For each non-seed activated node, find its shortest path from seeds.
	// We use a recursive CTE to compute hop distance from any seed.
	hopMap := make(map[string]int) // node_id -> min hop from any seed

	// Build list of seed IDs for SQL
	if len(seedIDs) > 0 {
		seedList := make([]string, 0, len(seedIDs))
		for id := range seedIDs {
			seedList = append(seedList, "'"+id+"'")
		}
		seedCondition := strings.Join(seedList, ",")

		hopQuery := fmt.Sprintf(`
			WITH RECURSIVE
			seed_nodes AS (
				SELECT node_id, 0 as hop FROM activation WHERE node_id IN (%s) AND intensity >= 0.9
			),
			prop(node_id, hop) AS (
				SELECT node_id, hop FROM seed_nodes
				UNION ALL
				SELECT 
					CASE WHEN e.source_id = p.node_id THEN e.target_id ELSE e.source_id END,
					p.hop + 1
				FROM prop p
				JOIN edges e ON (e.source_id = p.node_id OR e.target_id = p.node_id)
				WHERE p.hop < %d
			)
			SELECT node_id, MIN(hop) as min_hop
			FROM prop
			GROUP BY node_id
			HAVING node_id IN (SELECT node_id FROM activation WHERE intensity > %f)
		`, seedCondition, cfg.MaxHops, cfg.ActivationThreshold)

		hopRows, err := db.Query(hopQuery)
		if err == nil {
			defer hopRows.Close()
			for hopRows.Next() {
				var nid string
				var hop int
				if err := hopRows.Scan(&nid, &hop); err == nil {
					hopMap[nid] = hop
				}
			}
		}
	}

	// Group nodes by hop
	type HopGroup struct {
		Hop   int
		Nodes []FullNode
	}
	groups := make(map[int][]FullNode)
	for _, n := range allNodes {
		hop := 0
		if seedIDs[n.ID] {
			hop = 0
		} else if h, ok := hopMap[n.ID]; ok {
			hop = h
		} else {
			hop = -1 // unknown distance
		}
		groups[hop] = append(groups[hop], n)
	}

	// Find max hop for ordering
	var hopKeys []int
	for h := range groups {
		hopKeys = append(hopKeys, h)
	}
	sort.Ints(hopKeys)

	// Display
	for _, h := range hopKeys {
		nodesInHop, ok := groups[h]
		if !ok || len(nodesInHop) == 0 {
			continue
		}

		var title string
		switch h {
		case 0:
			title = "🎯 HOP 0 — Semillas (coincidencia directa)"
		case 1:
			title = "→ HOP 1 — Conexión directa"
		case 2:
			title = "→→ HOP 2 — Un intermediario"
		case 3:
			title = "→→→ HOP 3 — Dos intermediarios"
		default:
			title = fmt.Sprintf("→[%d] HOP %d", h, h)
		}
		fmt.Printf("  %s\n", title)
		fmt.Println(strings.Repeat("  ─", 20))

		for _, n := range nodesInHop {
			barLen := int(n.Intensity * 15)
			if barLen > 15 {
				barLen = 15
			}
			if barLen < 0 {
				barLen = 0
			}
			// Truncate label
			label := n.Label
			if len(label) > 55 {
				label = label[:55] + "…"
			}
			bar := strings.Repeat("▸", barLen)
			fmt.Printf("  %s %s  %s %.2f\n", n.Icon, label, green(bar), n.Intensity)
			if n.Content != "" && h <= 1 {
				// Only show content for close hops
				shortContent := n.Content
				if len(shortContent) > 70 {
					shortContent = shortContent[:70] + "…"
				}
				fmt.Printf("     └─ %s\n", shortContent)
			}
		}
		fmt.Println()
	}

	// Show unknown-distance nodes if any
	if unknown, ok := groups[-1]; ok && len(unknown) > 0 {
		fmt.Println("  ? HOP — Distancia no determinada:")
		for _, n := range unknown {
			label := n.Label
			if len(label) > 55 {
				label = label[:55] + "…"
			}
			fmt.Printf("  %s %s (%.2f)\n", n.Icon, label, n.Intensity)
		}
		fmt.Println()
	}

	// Stats
	fmt.Printf("📈 %d nodos en árbol de asociación (máx %d saltos)\n", len(allNodes), cfg.MaxHops)

	return nil
}
