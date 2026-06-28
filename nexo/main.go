package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LSTM global instance (RAM cognitiva)
var workingMemory *WorkingMemory

// getLSTMPath retorna la ruta del archivo de estado LSTM
func getLSTMPath() string {
	dir := os.Getenv("MEMORY_CORTEX_DIR")
	if dir == "" {
		dir = "/root/.memory-cortex"
	}
	return filepath.Join(dir, "lstm-state.json")
}

// initLSTM inicializa o carga la memoria de trabajo
func initLSTM() {
	workingMemory = NewWorkingMemory()
	stateFile := getLSTMPath()
	if err := workingMemory.LoadFromFile(stateFile); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Error cargando LSTM: %v\n", err)
	}
}

// saveLSTM persiste el estado LSTM
func saveLSTM() {
	if workingMemory == nil {
		return
	}
	if err := workingMemory.SaveToFile(getLSTMPath()); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Error guardando LSTM: %v\n", err)
	}
}

func main() {
	cfg := &DefaultConfig

	// Inicializar LSTM (RAM cognitiva)
	initLSTM()

	args := os.Args[1:]
	if len(args) == 0 {
		help()
		os.Exit(0)
	}

	// MCP server mode
	if args[0] == "--mcp" {
		RunMCP()
		return
	}

	// SSE server mode (HTTP)
	if args[0] == "--sse" {
		addr := ":8765"
		if len(args) > 1 {
			addr = args[1]
		}
		RunSSE(addr)
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	// Commands that don't need a database
	switch cmd {
	case "help", "--help", "-h":
		help()
		return
	case "init":
		runInit(cfg)
		return
	}

	// All other commands need a database
	db, err := OpenDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error al abrir base de datos: %v\n", err)
		fmt.Fprintf(os.Stderr, "  Ejecuta 'nexo init' primero\n")
		os.Exit(1)
	}
	defer db.Close()

	switch cmd {
	case "recall":
		query := strings.Join(cmdArgs, " ")
		if query == "" {
			fmt.Fprintln(os.Stderr, "Uso: nexo recall <query>")
			os.Exit(1)
		}
		// LSTM: Procesar input y expandir query
		entities := ExtractEntities(query)
		lstmResult := workingMemory.ProcessInput(query, entities)
		expandedQuery := lstmResult.ResolvedInput
		if err := Recall(db, cfg, expandedQuery); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		saveLSTM()

	case "recall-brief":
		query := strings.Join(cmdArgs, " ")
		if query == "" {
			return
		}
		// LSTM: Procesar input y expandir query
		entities := ExtractEntities(query)
		lstmResult := workingMemory.ProcessInput(query, entities)
		expandedQuery := lstmResult.ResolvedInput
		if err := RecallBrief(db, cfg, expandedQuery); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		saveLSTM()

	case "store":
		if len(cmdArgs) == 0 {
			fmt.Fprintln(os.Stderr, "Uso: nexo store <texto> [--title T] [--importance 0.5]")
			os.Exit(1)
		}
		text := strings.Join(cmdArgs, " ")
		title := ""
		importance := 0.5 // default medium importance
		if idx := strings.LastIndex(text, "--importance "); idx >= 0 {
			rest := text[idx+13:]
			endIdx := strings.IndexAny(rest, " \t")
			if endIdx < 0 {
				endIdx = len(rest)
			}
			fmt.Sscanf(rest[:endIdx], "%f", &importance)
			text = strings.TrimSpace(text[:idx])
		}
		if idx := strings.LastIndex(text, "--title "); idx >= 0 {
			title = strings.TrimSpace(text[idx+8:])
			text = strings.TrimSpace(text[:idx])
		}
		if err := Store(db, text, title, importance); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "consolidate":
		if err := Consolidate(db); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "stats":
		if err := Stats(db); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "handoff":
		state, err := Handoff(db, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error en handoff: %v\n", err)
			os.Exit(1)
		}
		// LSTM: Incluir estado de memoria de trabajo
		state.LSTMState = workingMemory.GetState()
		PrintHandoff(state)
		// Reset LSTM para nueva sesión
		workingMemory = NewWorkingMemory()
		saveLSTM()

	case "entities":
		text := strings.Join(cmdArgs, " ")
		if text == "" {
			fmt.Fprintln(os.Stderr, "Uso: nexo entities <texto>")
			os.Exit(1)
		}
		entities := ExtractEntities(text)
		fmt.Printf("Entidades extraídas de: %s\n", text)
		fmt.Println("---")
		for _, e := range entities {
			fmt.Printf("  %s: %s (confianza: %.2f)\n", e.Type, e.Value, e.Conf)
		}

	case "propagate":
		query := strings.Join(cmdArgs, " ")
		if query == "" {
			fmt.Fprintln(os.Stderr, "Uso: nexo propagate <query>")
			os.Exit(1)
		}
		fmt.Printf("Propagando: %s\n", query)
		result, err := SpreadActivation(db, cfg, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%d|%d\n", result.Seeds, result.Total)

	case "reflect":
		text := strings.Join(cmdArgs, " ")
		if text == "" {
			fmt.Fprintln(os.Stderr, "Uso: nexo reflect <insight>")
			os.Exit(1)
		}
		// LSTM: Procesar input y obtener contexto
		entities := ExtractEntities(text)
		lstmResult := workingMemory.ProcessInput(text, entities)
		// Usar contexto LSTM para enriquecer la reflexión
		enrichedText := text
		if lstmResult.ActiveTopic != "" {
			enrichedText = fmt.Sprintf("%s [tema activo: %s]", text, lstmResult.ActiveTopic)
		}
		if err := Reflect(db, enrichedText); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		saveLSTM()

	case "recall-deep":
		query := strings.Join(cmdArgs, " ")
		if query == "" {
			fmt.Fprintln(os.Stderr, "Uso: nexo recall-deep <query>")
			os.Exit(1)
		}
		if err := DeepRecall(db, cfg, query); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "journal":
		if err := Journal(db); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "prefs":
		if err := ShowPreferences(db); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "align":
		fmt.Println("🎯 Ejecutando análisis de alineamiento completo...")
		// Run a recall to update style vectors
		if len(cmdArgs) > 0 {
			query := strings.Join(cmdArgs, " ")
			UpdateStyleVector(db, query)
		}
		bias, err := GetAlignmentBias(db, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  Confianza: %.0f%%\n", bias.Confidence*100)
		fmt.Printf("  Estilo dominante: %s (%.2f)\n", bias.StyleBias.Dimension, bias.StyleBias.Value)
		fmt.Printf("  Nodos con peso ajustado: %d\n", len(bias.NodeWeights))

	case "pws-session":
		if err := ShowSessionSummary(db); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}

	case "pattern":
		if len(cmdArgs) == 0 {
			fmt.Fprintln(os.Stderr, "Uso: nexo pattern <texto>")
			os.Exit(1)
		}
		text := strings.Join(cmdArgs, " ")
		pattern, err := AnalyzePattern(db, text, PatternTypeQuery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Patrón registrado:\n")
		fmt.Printf("  ID: %s\n", pattern.ID)
		fmt.Printf("  Tipo: %s\n", pattern.PatternType)
		fmt.Printf("  Observaciones: %d\n", pattern.ObservationCount)
		fmt.Printf("  Peso: %.2f\n", pattern.Weight)

	case "speak":
		if len(cmdArgs) == 0 {
			fmt.Fprintln(os.Stderr, "Uso: nexo speak <texto> [--lang es]")
			os.Exit(1)
		}
		text := strings.Join(cmdArgs, " ")
		lang := "es"
		if idx := strings.Index(text, "--lang "); idx >= 0 {
			lang = text[idx+6:]
			text = strings.TrimSpace(text[:idx])
		}
		// Call Python TTS script
		cmd := exec.Command("python3", "/root/primerModelo/nexo/tts_cli.py", text, "-l", lang)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Error en TTS: %v\n", err)
			os.Exit(1)
		}

	case "lstm":
		// Mostrar estado de la memoria de trabajo
		state := workingMemory.GetState()
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════╗")
		fmt.Println("║     🧠 LSTM — RAM Cognitiva              ║")
		fmt.Println("║     Memoria de Trabajo Temporal          ║")
		fmt.Println("╚══════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("📊 Turnos: %d\n", state.TurnCount)
		fmt.Printf("🎯 Tema activo: %s\n", workingMemory.ActiveTopic)
		fmt.Printf("🔥 Intensidad tema: %.2f\n", workingMemory.TopicIntensity)
		fmt.Println()
		
		if len(state.Concepts) > 0 {
			fmt.Println("🏷️  CONCEPTOS ACTIVOS:")
			for _, c := range state.Concepts {
				bar := strings.Repeat("█", int(c.Weight*20))
				fmt.Printf("  • %s — %.2f %s\n", c.Label, c.Weight, bar)
			}
			fmt.Println()
		}
		
		if len(state.ContextWindow) > 0 {
			fmt.Println("📋 VENTANA DE CONTEXTO:")
			for _, e := range state.ContextWindow {
				preview := e.Content
				if len(preview) > 60 {
					preview = preview[:60] + "..."
				}
				role := "👤"
				if e.Role == "assistant" {
					role = "🤖"
				}
				fmt.Printf("  %s %s\n", role, preview)
			}
			fmt.Println()
		}
		
		fmt.Println("╚══════════════════════════════════════════╝")

	default:
		fmt.Fprintf(os.Stderr, "✗ Comando desconocido: %s\n\n", cmd)
		help()
		os.Exit(1)
	}
}

func runInit(cfg *DbConfig) {
	fmt.Println("⚠️  Esto borrará TODOS los datos existentes")

	// Remove existing database
	os.Remove(cfg.Path)

	db, err := OpenDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error al abrir BD: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := InitDB(db); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error al inicializar BD: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Base de datos inicializada: %s\n", cfg.Path)
}

func help() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        NEXO — Memoria Asociativa        ║")
	fmt.Println("║        Motor de memoria en Go           ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Uso: nexo <comando> [argumentos]")
	fmt.Println()
	fmt.Println("Comandos:")
	fmt.Println("  init              Inicializar nueva base de datos")
	fmt.Println("  recall <query>    Recuperar contexto asociativo")
	fmt.Println("  recall-brief <q>  Recuperar en formato breve (para agente)")
	fmt.Println("  store <texto>     Almacenar en memoria [--title T] [--importance 0.5]")
	fmt.Println("  reflect <texto>   Crear reflexión asociada al contexto actual")
	fmt.Println("  recall <query>    Recuperar contexto asociativo")
	fmt.Println("  recall-deep <q>   Recall profundo con cadenas multihop")
	fmt.Println("  journal           Guardar diario de sesión (resumen + cierre)")
	fmt.Println("  consolidate       Ejecutar consolidación (sueño)")
	fmt.Println("  stats             Mostrar estadísticas")
	fmt.Println("  handoff           🌅 Protocolo de despertar")
	fmt.Println("  lstm              🧠 Mostrar estado de memoria de trabajo")
	fmt.Println("  entities <texto>  Extraer entidades (debug)")
	fmt.Println("  propagate <query> Ejecutar propagación (debug)")
	fmt.Println("  prefs             Mostrar preferencias del usuario (PWS)")
	fmt.Println("  align <query>     Ejecutar análisis de alineamiento")
	fmt.Println("  pws-session       Consolidar y mostrar resumen de sesión")
	fmt.Println("  pattern <texto>   Registrar patrón de comportamiento")
	fmt.Println("  help              Mostrar esta ayuda")
	fmt.Println()
	fmt.Println("Variables de entorno:")
	fmt.Println("  MEMORY_CORTEX_DIR  Directorio de la BD (def: /root/.memory-cortex)")
	fmt.Println()
}
