package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// HandoffState represents the complete consolidated state at session start.
type HandoffState struct {
	Timestamp       int64
	TotalNodes      int
	TotalEdges      int
	TotalConsolidations int
	StrongNuclei    []Nucleus
	RecentEpisodes  []string
	RecentReflections []string
	RecentFacts     []string
	HotConcepts     []string
	Summary         string
	DreamJournal    string
	UserPreferences *UserPreferences
	StyleVector     map[string]StyleVector
	LSTMState       *LSTMState      // Memoria de trabajo
}

// Nucleus represents a strongly connected concept.
type Nucleus struct {
	Label       string
	Connections int
	Intensity   float64
}

// Handoff executes the wake-up protocol at session start.
// It reads the consolidated state and dream journal to prepare full context.
func Handoff(db *sql.DB, cfg *DbConfig) (*HandoffState, error) {
	state := &HandoffState{
		Timestamp: nowMs(),
	}

	// 1. Graph statistics
	db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&state.TotalNodes)
	db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&state.TotalEdges)
	db.QueryRow("SELECT COALESCE(CAST(value AS INTEGER), 0) FROM stats WHERE key = 'total_consolidations'").Scan(&state.TotalConsolidations)

	// 2. Strong nuclei: concepts with highest connection counts and activation
	nucleiRows, err := db.Query(`
		SELECT n.label, COUNT(e.id) as conns, COALESCE(a.intensity, 0) as intensity
		FROM nodes n
		JOIN edges e ON e.target_id = n.id OR e.source_id = n.id
		LEFT JOIN activation a ON a.node_id = n.id
		WHERE n.type = 'concept'
		GROUP BY n.id
		ORDER BY (COUNT(e.id) + COALESCE(a.intensity, 0) * 5) DESC
		LIMIT 8
	`)
	if err == nil {
		defer nucleiRows.Close()
		for nucleiRows.Next() {
			var n Nucleus
			if err := nucleiRows.Scan(&n.Label, &n.Connections, &n.Intensity); err == nil {
				state.StrongNuclei = append(state.StrongNuclei, n)
			}
		}
	}

	// 3. Recent episodes (last 10)
	epRows, err := db.Query(`
		SELECT label, content FROM nodes 
		WHERE type = 'episode' 
		ORDER BY created_at DESC 
		LIMIT 5
	`)
	if err == nil {
		defer epRows.Close()
		for epRows.Next() {
			var label, content string
			if err := epRows.Scan(&label, &content); err == nil {
				entry := label
				if content != "" {
					if len(content) > 80 {
						content = content[:80] + "..."
					}
					entry = entry + " — " + content
				}
				state.RecentEpisodes = append(state.RecentEpisodes, entry)
			}
		}
	}

	// 4. Recent reflections (last 3)
	refRows, err := db.Query(`
		SELECT label, content FROM nodes 
		WHERE type = 'reflection' 
		ORDER BY created_at DESC 
		LIMIT 3
	`)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			var label, content string
			if err := refRows.Scan(&label, &content); err == nil {
				entry := label
				if content != "" {
					if len(content) > 80 {
						content = content[:80] + "..."
					}
					entry = entry + " — " + content
				}
				state.RecentReflections = append(state.RecentReflections, entry)
			}
		}
	}

	// 5. Hot concepts: concepts with highest activation
	hotRows, err := db.Query(`
		SELECT n.label, a.intensity
		FROM activation a
		JOIN nodes n ON a.node_id = n.id
		WHERE a.intensity > ?
		ORDER BY a.intensity DESC
		LIMIT 8
	`, cfg.ActivationThreshold)
	if err == nil {
		defer hotRows.Close()
		for hotRows.Next() {
			var label string
			var intensity float64
			if err := hotRows.Scan(&label, &intensity); err == nil {
				state.HotConcepts = append(state.HotConcepts, fmt.Sprintf("%s (%.2f)", label, intensity))
			}
		}
	}

	// 6. Read dream journal
	dreamData, err := os.ReadFile(os.Getenv("MEMORY_CORTEX_DIR") + "/dreams/dream-latest.json")
	if err == nil {
		// Try to format it nicely, but keep raw if parse fails
		var dreamJSON interface{}
		if json.Unmarshal(dreamData, &dreamJSON) == nil {
			pretty, _ := json.MarshalIndent(dreamJSON, "", "  ")
			state.DreamJournal = string(pretty)
		} else {
			state.DreamJournal = string(dreamData)
		}
	}

	// 7. Generate summary
	state.Summary = buildSummary(state)

	// 8. PWS: Load user preferences and style vector
	prefs, err := GetUserPreferences(db)
	if err == nil {
		state.UserPreferences = prefs
	}
	styleVec, err := GetStyleVector(db)
	if err == nil {
		state.StyleVector = styleVec
	}

	return state, nil
}

func buildSummary(s *HandoffState) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("🧠 Memoria: %d nodos, %d aristas", s.TotalNodes, s.TotalEdges))
	parts = append(parts, fmt.Sprintf("🔄 Consolidaciones: %d", s.TotalConsolidations))

	if len(s.StrongNuclei) > 0 {
		var nuclei []string
		for _, n := range s.StrongNuclei[:min(5, len(s.StrongNuclei))] {
			nuclei = append(nuclei, fmt.Sprintf("%s (%d)", n.Label, n.Connections))
		}
		parts = append(parts, fmt.Sprintf("🏆 Núcleos fuertes: %s", strings.Join(nuclei, ", ")))
	}

	if len(s.HotConcepts) > 0 {
		parts = append(parts, fmt.Sprintf("🔥 Conceptos activos: %s", strings.Join(s.HotConcepts, ", ")))
	}

	return strings.Join(parts, " | ")
}

// PrintHandoff outputs the handoff state in a format for the agent to read.
func PrintHandoff(state *HandoffState) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║     🌅 NEXO — PROTOCOLO DE DESPERTAR               ║")
	fmt.Println("║     Estado Consolidado al Inicio de Sesión          ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("📊 %s\n", state.Summary)
	fmt.Println()

	// Nuclei
	if len(state.StrongNuclei) > 0 {
		fmt.Println("🏆 NÚCLEOS FUERTES (conceptos centrales):")
		for _, n := range state.StrongNuclei {
			bar := strings.Repeat("█", min(int(n.Intensity*20), 20))
			fmt.Printf("  • %s — %d conexiones  %s [%.2f]\n", n.Label, n.Connections, bar, n.Intensity)
		}
		fmt.Println()
	}

	// Recent reflections
	if len(state.RecentReflections) > 0 {
		fmt.Println("🧠 REFLEXIONES RECIENTES:")
		for _, r := range state.RecentReflections {
			fmt.Printf("  • %s\n", r)
		}
		fmt.Println()
	}

	// Hot concepts
	if len(state.HotConcepts) > 0 {
		fmt.Println("🔥 CONCEPTOS ACTIVOS (alta activación):")
		for _, h := range state.HotConcepts {
			fmt.Printf("  • %s\n", h)
		}
		fmt.Println()
	}

	// Recent episodes
	if len(state.RecentEpisodes) > 0 {
		fmt.Println("📋 EPISODIOS RECIENTES:")
		for i, ep := range state.RecentEpisodes {
			fmt.Printf("  %d. %s\n", i+1, ep)
		}
		fmt.Println()
	}

	// Dream journal
	if state.DreamJournal != "" {
		fmt.Println("🌙 DIARIO DE SUEÑOS (última consolidación):")
		// Try to parse the dream journal for a cleaner display
		var dreamMap map[string]interface{}
		if json.Unmarshal([]byte(state.DreamJournal), &dreamMap) == nil {
			if temas, ok := dreamMap["temas_recientes"]; ok {
				if temasArr, ok := temas.([]interface{}); ok {
					for _, t := range temasArr {
						fmt.Printf("  • %v\n", t)
					}
				}
			}
			if nucleos, ok := dreamMap["nucleos_fuertes"]; ok {
				if nucleosMap, ok := nucleos.(map[string]interface{}); ok {
					// Sort by value
					type kv struct {
						k string
						v float64
					}
					var sorted []kv
					for k, v := range nucleosMap {
						switch val := v.(type) {
						case float64:
							sorted = append(sorted, kv{k, val})
						}
					}
					sort.Slice(sorted, func(i, j int) bool {
						return sorted[i].v > sorted[j].v
					})
					if len(sorted) > 0 {
						var parts []string
						for _, kv := range sorted {
							parts = append(parts, fmt.Sprintf("%s: %.0f", kv.k, kv.v))
						}
						fmt.Printf("  Núcleos: %s\n", strings.Join(parts, ", "))
					}
				}
			}
		}
		fmt.Println()
	}

	// User preferences (PWS)
	if state.UserPreferences != nil && state.UserPreferences.Confidence > 0.1 {
		fmt.Println("🎯 PREFERENCIAS DEL USUARIO (PWS):")
		fmt.Printf("  Longitud: %s | Detalle: %s | Tono: %s\n",
			state.UserPreferences.PreferredResponseLength,
			state.UserPreferences.PreferredDetailLevel,
			state.UserPreferences.PreferredTone)
		fmt.Printf("  Confianza: %.0f%%\n", state.UserPreferences.AlignmentScore*100)

		// Show top style dimensions
		if state.StyleVector != nil {
			var topStyles []string
			for dim, sv := range state.StyleVector {
				if sv.Samples > 5 {
					topStyles = append(topStyles, fmt.Sprintf("%s: %.2f", dim, sv.Value))
				}
			}
			if len(topStyles) > 0 {
				fmt.Printf("  Estilo: %s\n", strings.Join(topStyles, ", "))
			}
		}
		fmt.Println()
	}

	// LSTM: Memoria de trabajo
	if state.LSTMState != nil && state.LSTMState.TurnCount > 0 {
		fmt.Println("🧠 MEMORIA DE TRABAJO (LSTM):")
		fmt.Printf("  Turnos anteriores: %d\n", state.LSTMState.TurnCount)
		if state.LSTMState.ActiveTopic != "" {
			fmt.Printf("  Tema activo: %s\n", state.LSTMState.ActiveTopic)
		}
		if len(state.LSTMState.Concepts) > 0 {
			var concepts []string
			for _, c := range state.LSTMState.Concepts {
				concepts = append(concepts, fmt.Sprintf("%s (%.2f)", c.Label, c.Weight))
			}
			fmt.Printf("  Conceptos activos: %s\n", strings.Join(concepts, ", "))
		}
		fmt.Println()
	}

	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
