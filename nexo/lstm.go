package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

// ============================================================
//  LSTM — Working Memory / RAM Cognitiva
//  Memoria de trabajo temporal entre entrada y administrador
// ============================================================

const (
	LSTMWindowSize    = 10    // Últimas N interacciones en ventana
	LSTMHiddenSize    = 64    // Dimensión del estado oculto
	LSTMConceptSlots  = 8     // Slots para conceptos activos
	LSTMRefThreshold  = 0.6   // Umbral para resolver referencias
)

// WorkingMemory implementa una memoria de trabajo tipo LSTM
type WorkingMemory struct {
	mu sync.RWMutex
	
	// Estado oculto (RAM cognitiva)
	HiddenState    []float64        `json:"hidden_state"`
	CellState      []float64        `json:"cell_state"`
	
	// Foco temático
	ActiveTopic    string           `json:"active_topic"`
	TopicIntensity float64          `json:"topic_intensity"`
	
	// Conceptos en memoria de trabajo
	ActiveConcepts []ConceptSlot    `json:"active_concepts"`
	
	// Ventana de contexto reciente
	ContextWindow  []ContextEntry   `json:"context_window"`
	
	// Referencias pendientes
	PendingRefs    []Reference      `json:"pending_refs"`
	
	// Intención reciente
	LastIntent     string           `json:"last_intent"`
	LastEntities   []string         `json:"last_entities"`
	
	// Timestamps
	CreatedAt      int64            `json:"created_at"`
	LastUpdated    int64            `json:"last_updated"`
	TurnCount      int              `json:"turn_count"`
}

// ConceptSlot representa un concepto activo en la memoria de trabajo
type ConceptSlot struct {
	Label      string  `json:"label"`
	Weight     float64 `json:"weight"`
	Frequency  int     `json:"frequency"`
	LastSeen   int64   `json:"last_seen"`
	Decay      float64 `json:"decay"`
}

// ContextEntry representa una interacción en la ventana
type ContextEntry struct {
	Timestamp  int64    `json:"timestamp"`
	Role       string   `json:"role"`      // "user" o "assistant"
	Content    string   `json:"content"`
	Topics     []string `json:"topics"`
	Entities   []string `json:"entities"`
}

// Reference representa una referencia pendiente de resolver
type Reference struct {
	Type       string  `json:"type"`       // "pronombre", "demostrativo", "elision"
	Raw        string  `json:"raw"`        // "eso", "aquello", "unirlos"
	Position   int     `json:"position"`
	Confidence float64 `json:"confidence"`
	Resolved   bool    `json:"resolved"`
	ResolvedTo string  `json:"resolved_to,omitempty"`
}

// LSTMState estado serializado para persistencia
type LSTMState struct {
	HiddenState   []float64      `json:"h"`
	CellState     []float64      `json:"c"`
	ActiveTopic   string         `json:"topic"`
	Concepts      []ConceptSlot  `json:"concepts"`
	ContextWindow []ContextEntry `json:"window"`
	TurnCount     int            `json:"turns"`
}

// NewWorkingMemory crea una nueva instancia de memoria de trabajo
func NewWorkingMemory() *WorkingMemory {
	return &WorkingMemory{
		HiddenState:    make([]float64, LSTMHiddenSize),
		CellState:      make([]float64, LSTMHiddenSize),
		ActiveConcepts: make([]ConceptSlot, 0, LSTMConceptSlots),
		ContextWindow:  make([]ContextEntry, 0, LSTMWindowSize),
		PendingRefs:    make([]Reference, 0),
		CreatedAt:      time.Now().UnixMilli(),
		LastUpdated:    time.Now().UnixMilli(),
		TurnCount:      0,
	}
}

// ProcessInput procesa una entrada del usuario y actualiza la memoria de trabajo
func (wm *WorkingMemory) ProcessInput(input string, entities []Entity) *LSTMResult {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	now := time.Now().UnixMilli()
	
	// 1. Extraer topics y entidades del input
	topics := extractTopics(input)
	entityLabels := make([]string, len(entities))
	for i, e := range entities {
		entityLabels[i] = e.Value
	}
	
	// 2. Detectar referencias pendientes
	refs := detectReferences(input, len(wm.ContextWindow))
	wm.PendingRefs = append(wm.PendingRefs, refs...)
	
	// 3. Resolver referencias contra contexto reciente
	resolved := wm.resolveReferences(input, refs)
	
	// 4. Actualizar estado oculto (simula gates LSTM)
	wm.updateHiddenState(input, topics, entityLabels)
	
	// 5. Actualizar conceptos activos
	wm.updateActiveConcepts(topics, entityLabels, now)
	
	// 6. Agregar a ventana de contexto
	entry := ContextEntry{
		Timestamp: now,
		Role:      "user",
		Content:   input,
		Topics:    topics,
		Entities:  entityLabels,
	}
	wm.ContextWindow = append(wm.ContextWindow, entry)
	if len(wm.ContextWindow) > LSTMWindowSize {
		wm.ContextWindow = wm.ContextWindow[1:]
	}
	
	// 7. Detectar continuidad temática
	continuity := wm.detectTopicContinuity(topics)
	
	// 8. Actualizar intención
	wm.LastIntent = wm.inferIntent(input, topics)
	wm.LastEntities = entityLabels
	wm.LastUpdated = now
	wm.TurnCount++
	
	// 9. Determinar tema activo dominante
	if len(topics) > 0 && continuity > 0.5 {
		wm.ActiveTopic = topics[0]
		wm.TopicIntensity = math.Min(1.0, wm.TopicIntensity+0.2)
	} else if len(topics) > 0 {
		wm.ActiveTopic = topics[0]
		wm.TopicIntensity = 0.5
	}
	
	return &LSTMResult{
		ResolvedInput:  resolved,
		ActiveTopic:    wm.ActiveTopic,
		TopicIntensity: wm.TopicIntensity,
		Concepts:       wm.getActiveConceptLabels(),
		Continuity:     continuity,
		Intent:         wm.LastIntent,
		References:     resolved,
	}
}

// ProcessAssistantOutput registra la respuesta del asistente
func (wm *WorkingMemory) ProcessAssistantOutput(output string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	now := time.Now().UnixMilli()
	
	topics := extractTopics(output)
	
	entry := ContextEntry{
		Timestamp: now,
		Role:      "assistant",
		Content:   output,
		Topics:    topics,
	}
	wm.ContextWindow = append(wm.ContextWindow, entry)
	if len(wm.ContextWindow) > LSTMWindowSize {
		wm.ContextWindow = wm.ContextWindow[1:]
	}
	
	wm.LastUpdated = now
}

// GetContextForRecall retorna el contexto optimizado para recall
func (wm *WorkingMemory) GetContextForRecall(query string) *RecallContext {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	// Combinar query con contexto activo
	expandedQuery := query
	
	// Agregar tema activo si es relevante
	if wm.ActiveTopic != "" && wm.TopicIntensity > 0.3 {
		expandedQuery += " " + wm.ActiveTopic
	}
	
	// Agregar conceptos activos de mayor peso
	for _, c := range wm.ActiveConcepts {
		if c.Weight > 0.5 {
			expandedQuery += " " + c.Label
		}
	}
	
	// Resolver referencias en la query
	for _, ref := range wm.PendingRefs {
		if ref.Resolved && ref.ResolvedTo != "" {
			expandedQuery = strings.Replace(expandedQuery, ref.Raw, ref.ResolvedTo, -1)
		}
	}
	
	return &RecallContext{
		OriginalQuery: query,
		ExpandedQuery: expandedQuery,
		ActiveTopic:   wm.ActiveTopic,
		ActiveConcepts: wm.getActiveConceptLabels(),
		WindowSummary: wm.summarizeWindow(),
	}
}

// GetState retorna el estado LSTM para persistencia
func (wm *WorkingMemory) GetState() *LSTMState {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	return &LSTMState{
		HiddenState:   wm.HiddenState,
		CellState:     wm.CellState,
		ActiveTopic:   wm.ActiveTopic,
		Concepts:      wm.ActiveConcepts,
		ContextWindow: wm.ContextWindow,
		TurnCount:     wm.TurnCount,
	}
}

// LoadState carga estado LSTM desde persistencia
func (wm *WorkingMemory) LoadState(state *LSTMState) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	if state == nil {
		return
	}
	
	wm.HiddenState = state.HiddenState
	wm.CellState = state.CellState
	wm.ActiveTopic = state.ActiveTopic
	wm.ActiveConcepts = state.Concepts
	wm.ContextWindow = state.ContextWindow
	wm.TurnCount = state.TurnCount
}

// SaveToFile persiste el estado LSTM a disco
func (wm *WorkingMemory) SaveToFile(path string) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	data, err := json.MarshalIndent(wm.GetState(), "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling LSTM state: %w", err)
	}
	
	return os.WriteFile(path, data, 0644)
}

// LoadFromFile carga estado LSTM desde disco
func (wm *WorkingMemory) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Archivo no existe, usar estado nuevo
		}
		return fmt.Errorf("error reading LSTM state: %w", err)
	}
	
	var state LSTMState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("error unmarshaling LSTM state: %w", err)
	}
	
	wm.LoadState(&state)
	return nil
}

// ============================================================
//  Funciones internas de procesamiento
// ============================================================

// updateHiddenState simula los gates de una LSTM real
func (wm *WorkingMemory) updateHiddenState(input string, topics, entities []string) {
	// Simplified LSTM gates (no necesitamos pesos entrenados)
	// Solo modelamos el comportamiento de memoria de trabajo
	
	inputNorm := normalizeInput(input)
	
	// Forget gate: qué olvidar del cell state
	forgetGate := sigmoid(float64(len(wm.ContextWindow)) / float64(LSTMWindowSize))
	
	// Input gate: qué información nueva guardar
	inputGate := math.Min(1.0, float64(len(topics)+len(entities))*0.2)
	
	// Update cell state
	for i := range wm.CellState {
		// Olvidar info antigua
		wm.CellState[i] *= forgetGate
		// Agregar info nueva (basada en hash del input)
		if i < len(inputNorm) {
			wm.CellState[i] += inputNorm[i] * inputGate
		}
		// Decay natural
		wm.CellState[i] *= 0.95
	}
	
	// Update hidden state (combinación de cell state y input)
	for i := range wm.HiddenState {
		wm.HiddenState[i] = tanh(wm.CellState[i]) * 0.8 + wm.HiddenState[i]*0.2
	}
}

// updateActiveConcepts actualiza los slots de conceptos activos
func (wm *WorkingMemory) updateActiveConcepts(topics, entities []string, now int64) {
	// Decaimiento natural de todos los conceptos
	for i := range wm.ActiveConcepts {
		elapsed := float64(now-wm.ActiveConcepts[i].LastSeen) / 1000.0 // seconds
		wm.ActiveConcepts[i].Weight *= math.Exp(-elapsed / 60.0) // decay en 1 minuto
		wm.ActiveConcepts[i].Decay = elapsed
	}
	
	// Agregar/actualizar topics
	for _, t := range topics {
		wm.upsertConcept(t, 0.8, now)
	}
	
	// Agregar/actualizar entidades (mayor prioridad)
	for _, e := range entities {
		wm.upsertConcept(e, 1.0, now)
	}
	
	// Mantener solo los top LSTMConceptSlots
	if len(wm.ActiveConcepts) > LSTMConceptSlots {
		// Ordenar por peso descendente
		sortByWeight(wm.ActiveConcepts)
		wm.ActiveConcepts = wm.ActiveConcepts[:LSTMConceptSlots]
	}
}

// upsertConcept actualiza o inserta un concepto
func (wm *WorkingMemory) upsertConcept(label string, weight float64, now int64) {
	normalized := strings.ToLower(strings.TrimSpace(label))
	
	for i, c := range wm.ActiveConcepts {
		if strings.ToLower(c.Label) == normalized {
			wm.ActiveConcepts[i].Weight = math.Min(1.0, c.Weight+weight*0.3)
			wm.ActiveConcepts[i].Frequency++
			wm.ActiveConcepts[i].LastSeen = now
			return
		}
	}
	
	// Nuevo concepto
	wm.ActiveConcepts = append(wm.ActiveConcepts, ConceptSlot{
		Label:     label,
		Weight:    weight,
		Frequency: 1,
		LastSeen:  now,
		Decay:     0,
	})
}

// detectReferences identifica referencias en el input
func detectReferences(input string, windowPos int) []Reference {
	var refs []Reference
	lower := strings.ToLower(input)
	
	// Pronombres demostrativos
	demostrativos := []string{"eso", "aquello", "ello", "esto", "aquí", "allí"}
	for _, d := range demostrativos {
		if idx := strings.Index(lower, d); idx >= 0 {
			refs = append(refs, Reference{
				Type:       "demostrativo",
				Raw:        d,
				Position:   idx,
				Confidence: 0.7,
			})
		}
	}
	
	// Elisiones verbales
	type ElisionEntry struct {
		Pattern string
		Parts   []string
	}
	elisiones := []ElisionEntry{
		{Pattern: "unirlos", Parts: []string{"unir", "los"}},
		{Pattern: "hacerlo", Parts: []string{"hacer", "lo"}},
		{Pattern: "decirlo", Parts: []string{"decir", "lo"}},
		{Pattern: "verlo", Parts: []string{"ver", "lo"}},
		{Pattern: "usarlo", Parts: []string{"usar", "lo"}},
	}
	
	for _, e := range elisiones {
		if strings.Contains(lower, e.Pattern) {
			refs = append(refs, Reference{
				Type:       "elision",
				Raw:        e.Pattern,
				Position:   strings.Index(lower, e.Pattern),
				Confidence: 0.8,
			})
		}
	}
	
	// Referencias contextuales
	contextuales := []string{"el otro", "lo otro", "eso otro", "más", "también", "después"}
	for _, c := range contextuales {
		if strings.Contains(lower, c) {
			refs = append(refs, Reference{
				Type:       "contextual",
				Raw:        c,
				Position:   strings.Index(lower, c),
				Confidence: 0.5,
			})
		}
	}
	
	return refs
}

// resolveReferences intenta resolver referencias contra el contexto
func (wm *WorkingMemory) resolveReferences(input string, refs []Reference) string {
	resolved := input
	
	for i, ref := range refs {
		if ref.Confidence < LSTMRefThreshold {
			continue
		}
		
		// Buscar en contexto reciente
		for j := len(wm.ContextWindow) - 1; j >= 0; j-- {
			entry := wm.ContextWindow[j]
			if entry.Role == "user" && len(entry.Entities) > 0 {
				// La referencia apunta a la última entidad mencionada
				lastEntity := entry.Entities[len(entry.Entities)-1]
				refs[i].Resolved = true
				refs[i].ResolvedTo = lastEntity
				resolved = strings.Replace(resolved, ref.Raw, lastEntity, 1)
				break
			}
			if entry.Role == "user" && len(entry.Topics) > 0 {
				// Si no hay entidades, usar el topic
				lastTopic := entry.Topics[len(entry.Topics)-1]
				refs[i].Resolved = true
				refs[i].ResolvedTo = lastTopic
				resolved = strings.Replace(resolved, ref.Raw, lastTopic, 1)
				break
			}
		}
	}
	
	return resolved
}

// detectTopicContinuity mide qué tan seguido es el mismo tema
func (wm *WorkingMemory) detectTopicContinuity(newTopics []string) float64 {
	if len(wm.ContextWindow) == 0 || len(newTopics) == 0 {
		return 0.0
	}
	
	// Contar cuántos topics recientes son iguales
	matches := 0
	total := 0
	
	for i := len(wm.ContextWindow) - 1; i >= 0 && i >= len(wm.ContextWindow)-3; i-- {
		entry := wm.ContextWindow[i]
		if entry.Role != "user" {
			continue
		}
		total++
		for _, t := range entry.Topics {
			for _, nt := range newTopics {
				if strings.EqualFold(t, nt) {
					matches++
					break
				}
			}
		}
	}
	
	if total == 0 {
		return 0.0
	}
	
	return float64(matches) / float64(total)
}

// inferIntent infiere la intención del usuario
func (wm *WorkingMemory) inferIntent(input string, topics []string) string {
	lower := strings.ToLower(input)
	
	if strings.HasSuffix(lower, "?") {
		return "question"
	}
	if len(topics) > 0 && wm.ActiveTopic != "" {
		// Detectar cambio de tema
		for _, t := range topics {
			if !strings.EqualFold(t, wm.ActiveTopic) {
				return "topic_shift"
			}
		}
		return "continue_topic"
	}
	
	return "statement"
}

// summarizeWindow crea un resumen de la ventana de contexto
func (wm *WorkingMemory) summarizeWindow() string {
	if len(wm.ContextWindow) == 0 {
		return "(sin contexto)"
	}
	
	var parts []string
	for _, e := range wm.ContextWindow {
		preview := e.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", e.Role, preview))
	}
	
	return strings.Join(parts, " → ")
}

// getActiveConceptLabels retorna labels de conceptos activos
func (wm *WorkingMemory) getActiveConceptLabels() []string {
	labels := make([]string, len(wm.ActiveConcepts))
	for i, c := range wm.ActiveConcepts {
		labels[i] = c.Label
	}
	return labels
}

// ============================================================
//  Funciones auxiliares
// ============================================================

func normalizeInput(input string) []float64 {
	// Hash simple para crear vector numérico del input
	result := make([]float64, LSTMHiddenSize)
	for i, c := range input {
		idx := i % LSTMHiddenSize
		result[idx] += float64(c) / 1000.0
	}
	// Normalizar
	sum := 0.0
	for _, v := range result {
		sum += v * v
	}
	if sum > 0 {
		norm := math.Sqrt(sum)
		for i := range result {
			result[i] /= norm
		}
	}
	return result
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func tanh(x float64) float64 {
	return math.Tanh(x)
}

func sortByWeight(slots []ConceptSlot) {
	for i := 1; i < len(slots); i++ {
		for j := i; j > 0 && slots[j].Weight > slots[j-1].Weight; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}
}

func extractTopics(text string) []string {
	// Extraer palabras clave como topics (simplificado)
	words := strings.Fields(strings.ToLower(text))
	topics := make([]string, 0)
	seen := make(map[string]bool)
	
	stopWords := map[string]bool{
		"el": true, "la": true, "los": true, "las": true,
		"un": true, "una": true, "de": true, "del": true,
		"en": true, "con": true, "por": true, "para": true,
		"que": true, "es": true, "son": true, "y": true,
		"o": true, "pero": true, "como": true, "más": true,
		"este": true, "esta": true, "esto": true, "ese": true,
		"mi": true, "tu": true, "su": true, "al": true,
	}
	
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len(w) > 3 && !stopWords[w] && !seen[w] {
			topics = append(topics, w)
			seen[w] = true
		}
		if len(topics) >= 5 {
			break
		}
	}
	
	return topics
}

// ============================================================
//  Tipos de resultado
// ============================================================

// LSTMResult resultado del procesamiento LSTM
type LSTMResult struct {
	ResolvedInput  string   `json:"resolved_input"`
	ActiveTopic    string   `json:"active_topic"`
	TopicIntensity float64  `json:"topic_intensity"`
	Concepts       []string `json:"concepts"`
	Continuity     float64  `json:"continuity"`
	Intent         string   `json:"intent"`
	References     string   `json:"references"`
}

// RecallContext contexto expandido para recall
type RecallContext struct {
	OriginalQuery  string   `json:"original_query"`
	ExpandedQuery  string   `json:"expanded_query"`
	ActiveTopic    string   `json:"active_topic"`
	ActiveConcepts []string `json:"active_concepts"`
	WindowSummary  string   `json:"window_summary"`
}
