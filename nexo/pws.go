package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

// Style dimensions for user behavior profiling
var StyleDimensions = []string{
	"verbosity",
	"formality",
	"technicality",
	"directness",
	"emotionality",
	"complexity",
	"frequency",
}

// Pattern types for user behavior tracking
const (
	PatternTypeQuery  = "query"
	PatternTypeStore  = "store"
	PatternTypeReflect = "reflect"
	PatternTypeStyle  = "style"
)

// UserPattern represents a detected behavior pattern
type UserPattern struct {
	ID               string  `json:"id"`
	PatternType      string  `json:"pattern_type"`
	ObservationCount int     `json:"observation_count"`
	Weight           float64 `json:"weight"`
	FirstSeen        int64   `json:"first_seen"`
	LastSeen         int64   `json:"last_seen"`
	Data             PatternData `json:"data"`
}

// PatternData holds extracted features from user behavior
type PatternData struct {
	AvgLength      float64  `json:"avg_length"`
	Complexity     float64  `json:"complexity"`
	Entities       []string `json:"entities"`
	Keywords       []string `json:"keywords"`
	SentenceCount  int      `json:"sentence_count"`
	QuestionRatio  float64  `json:"question_ratio"`
	CommandRatio   float64  `json:"command_ratio"`
}

// StyleVector represents the user's behavioral style across dimensions
type StyleVector struct {
	Dimension string  `json:"dimension"`
	Value     float64 `json:"value"`     // 0.0 to 1.0
	Samples   int     `json:"samples"`
}

// UserPreferences holds calculated preferences
type UserPreferences struct {
	PreferredResponseLength string  `json:"preferred_response_length"` // "short", "medium", "long"
	PreferredDetailLevel    string  `json:"preferred_detail_level"`    // "brief", "moderate", "comprehensive"
	PreferredTone           string  `json:"preferred_tone"`            // "formal", "casual", "technical"
	AlignmentScore          float64 `json:"alignment_score"`
	Confidence              float64 `json:"confidence"`
}

// AlignmentBias represents the bias to apply during recall/propagation
type AlignmentBias struct {
	NodeWeights map[string]float64 `json:"node_weights"` // node_id -> multiplier
	StyleBias   StyleVector        `json:"style_bias"`
	Confidence  float64            `json:"confidence"`
}

// AnalyzePattern detects and records a user behavior pattern
func AnalyzePattern(db *sql.DB, input string, operationType string) (*UserPattern, error) {
	now := time.Now().Unix()

	// Extract features from input
	data := extractPatternData(input)

	// Calculate pattern fingerprint (simple hash of key features)
	fingerprint := calculateFingerprint(data)

	// Check if pattern exists
	var existingID string
	var obsCount int
	var oldWeight float64
	err := db.QueryRow(
		"SELECT id, observation_count, weight FROM user_patterns WHERE pattern_type = ? AND pattern_data LIKE ?",
		operationType, "%"+fingerprint+"%",
	).Scan(&existingID, &obsCount, &oldWeight)

	if err == nil {
		// Update existing pattern
		newWeight := math.Min(oldWeight+0.05, 2.0)
		_, err = db.Exec(
			"UPDATE user_patterns SET observation_count = observation_count + 1, last_seen = ?, weight = ?, pattern_data = ? WHERE id = ?",
			now, newWeight, marshalPatternData(data), existingID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating pattern: %w", err)
		}
		return &UserPattern{
			ID: existingID, PatternType: operationType,
			ObservationCount: obsCount + 1, Weight: newWeight,
			FirstSeen: now, LastSeen: now, Data: data,
		}, nil
	}

	// Create new pattern
	id := makeID("pattern")
	_, err = db.Exec(
		"INSERT INTO user_patterns (id, pattern_type, pattern_data, observation_count, first_seen, last_seen, weight) VALUES (?, ?, ?, 1, ?, ?, 1.0)",
		id, operationType, marshalPatternData(data), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting pattern: %w", err)
	}

	return &UserPattern{
		ID: id, PatternType: operationType,
		ObservationCount: 1, Weight: 1.0,
		FirstSeen: now, LastSeen: now, Data: data,
	}, nil
}

// UpdateStyleVector updates the user's style dimensions based on new input
func UpdateStyleVector(db *sql.DB, text string) error {
	now := time.Now().Unix()
	features := extractStyleFeatures(text)

	for dim, value := range features {
		// Get current value
		var currentVal float64
		var samples int
		err := db.QueryRow(
			"SELECT value, sample_size FROM style_vectors WHERE dimension = ?", dim,
		).Scan(&currentVal, &samples)

		if err == nil {
			// Exponential moving average: new = old * 0.9 + observed * 0.1
			newSamples := samples + 1
			alpha := 1.0 / float64(newSamples)
			newVal := currentVal*(1-alpha) + value*alpha
			_, err = db.Exec(
				"UPDATE style_vectors SET value = ?, sample_size = ?, updated_at = ? WHERE dimension = ?",
				newVal, newSamples, now, dim,
			)
			if err != nil {
				return fmt.Errorf("updating style vector: %w", err)
			}
		} else {
			// Insert new dimension
			id := makeID("style")
			_, err = db.Exec(
				"INSERT INTO style_vectors (id, dimension, value, sample_size, updated_at) VALUES (?, ?, ?, 1, ?)",
				id, dim, value, now,
			)
			if err != nil {
				return fmt.Errorf("inserting style vector: %w", err)
			}
		}
	}

	return nil
}

// GetStyleVector retrieves the current style vector
func GetStyleVector(db *sql.DB) (map[string]StyleVector, error) {
	rows, err := db.Query("SELECT dimension, value, sample_size FROM style_vectors")
	if err != nil {
		return nil, fmt.Errorf("querying style vectors: %w", err)
	}
	defer rows.Close()

	result := make(map[string]StyleVector)
	for rows.Next() {
		var sv StyleVector
		if err := rows.Scan(&sv.Dimension, &sv.Value, &sv.Samples); err != nil {
			return nil, fmt.Errorf("scanning style vector: %w", err)
		}
		result[sv.Dimension] = sv
	}
	return result, nil
}

// GetAlignmentBias calculates the alignment bias for recall/propagation
func GetAlignmentBias(db *sql.DB, contextNodes []string) (*AlignmentBias, error) {
	styleVector, err := GetStyleVector(db)
	if err != nil {
		return nil, err
	}

	// Calculate confidence based on sample size
	totalSamples := 0
	for _, sv := range styleVector {
		totalSamples += sv.Samples
	}
	confidence := math.Min(float64(totalSamples)/100.0, 1.0) // Max confidence at 100 samples

	// Calculate node weights based on style preferences
	nodeWeights := make(map[string]float64)
	for _, nodeID := range contextNodes {
		weight := 1.0

		// Adjust based on verbosity preference
		if sv, ok := styleVector["verbosity"]; ok && sv.Samples > 5 {
			// High verbosity preference: boost nodes with longer content
			weight *= 1.0 + (sv.Value-0.5)*0.3
		}

		// Adjust based on technicality preference
		if sv, ok := styleVector["technicality"]; ok && sv.Samples > 5 {
			// High technicality: boost concept nodes
			weight *= 1.0 + (sv.Value-0.5)*0.2
		}

		// Adjust based on directness preference
		if sv, ok := styleVector["directness"]; ok && sv.Samples > 5 {
			// High directness: boost fact nodes
			weight *= 1.0 + (sv.Value-0.5)*0.15
		}

		nodeWeights[nodeID] = math.Max(0.5, math.Min(2.0, weight))
	}

	// Get primary style bias (dominant dimension)
	primaryDim := "directness"
	primaryVal := 0.5
	for dim, sv := range styleVector {
		if sv.Samples > 10 && math.Abs(sv.Value-0.5) > math.Abs(primaryVal-0.5) {
			primaryDim = dim
			primaryVal = sv.Value
		}
	}

	return &AlignmentBias{
		NodeWeights: nodeWeights,
		StyleBias: StyleVector{
			Dimension: primaryDim,
			Value:     primaryVal,
			Samples:   totalSamples,
		},
		Confidence: confidence,
	}, nil
}

// InferIntent attempts to infer user intent from context
func InferIntent(db *sql.DB, input string, contextNodes []string) (string, float64, error) {
	inputLower := strings.ToLower(input)

	// Intent patterns
	intentPatterns := map[string][]string{
		"question":    {"?", "qué", "cómo", "cuándo", "dónde", "por qué", "cuánto", "quién", "cuál"},
		"command":     {"haz", "ejecuta", "crea", "agrega", "elimina", "modifica", "actualiza", "guarda"},
		"request":     {"necesito", "quiero", "puedes", "podrías", "me gustaría", "dame", "proporciona"},
		"reflection":  {"pienso", "creo", "opino", "considero", "analizo", "reflexiono"},
		"exploration": {"explora", "busca", "encuentra", "investiga", "analiza", "revisa"},
	}

	bestIntent := "unknown"
	bestScore := 0.0

	for intent, keywords := range intentPatterns {
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(inputLower, kw) {
				score += 1.0
			}
		}
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	// Normalize score
	confidence := math.Min(bestScore/3.0, 1.0)

	// Store intent pattern
	if bestIntent != "unknown" {
		_, _ = AnalyzePattern(db, input, PatternTypeQuery)
	}

	return bestIntent, confidence, nil
}

// GetUserPreferences calculates and returns current user preferences
func GetUserPreferences(db *sql.DB) (*UserPreferences, error) {
	styleVector, err := GetStyleVector(db)
	if err != nil {
		return nil, err
	}

	prefs := &UserPreferences{
		PreferredResponseLength: "medium",
		PreferredDetailLevel:    "moderate",
		PreferredTone:           "casual",
		AlignmentScore:          0.5,
		Confidence:              0.0,
	}

	totalSamples := 0
	for _, sv := range styleVector {
		totalSamples += sv.Samples
	}

	if totalSamples < 5 {
		return prefs, nil
	}

	// Determine preferences from style vector
	if sv, ok := styleVector["verbosity"]; ok {
		switch {
		case sv.Value < 0.3:
			prefs.PreferredResponseLength = "short"
		case sv.Value > 0.7:
			prefs.PreferredResponseLength = "long"
		default:
			prefs.PreferredResponseLength = "medium"
		}
	}

	if sv, ok := styleVector["complexity"]; ok {
		switch {
		case sv.Value < 0.3:
			prefs.PreferredDetailLevel = "brief"
		case sv.Value > 0.7:
			prefs.PreferredDetailLevel = "comprehensive"
		default:
			prefs.PreferredDetailLevel = "moderate"
		}
	}

	if sv, ok := styleVector["formality"]; ok {
		if sv.Value > 0.6 {
			prefs.PreferredTone = "formal"
		} else if sv.Value < 0.4 {
			prefs.PreferredTone = "casual"
		} else {
			prefs.PreferredTone = "technical"
		}
	}

	// Calculate alignment score (how well we understand the user)
	prefs.AlignmentScore = math.Min(float64(totalSamples)/50.0, 1.0)
	prefs.Confidence = prefs.AlignmentScore

	return prefs, nil
}

// ShowPreferences displays current user preferences in formatted output
func ShowPreferences(db *sql.DB) error {
	prefs, err := GetUserPreferences(db)
	if err != nil {
		return err
	}

	styleVector, err := GetStyleVector(db)
	if err != nil {
		return err
	}

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║     🎯 PWS — Preferencias del Usuario           ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	// Style vector
	fmt.Println("📊 Vector de Estilo:")
	for _, dim := range StyleDimensions {
		if sv, ok := styleVector[dim]; ok {
			bar := strings.Repeat("█", int(sv.Value*20))
			empty := strings.Repeat("░", 20-int(sv.Value*20))
			fmt.Printf("  %-15s %s%s %.2f (%d samples)\n", dim, bar, empty, sv.Value, sv.Samples)
		}
	}
	fmt.Println()

	// Preferences
	fmt.Println("⚙️  Preferencias Detectadas:")
	fmt.Printf("  Longitud de respuesta: %s\n", prefs.PreferredResponseLength)
	fmt.Printf("  Nivel de detalle:      %s\n", prefs.PreferredDetailLevel)
	fmt.Printf("  Tono:                  %s\n", prefs.PreferredTone)
	fmt.Println()

	// Confidence
	fmt.Printf("🎯 Confianza del alineamiento: %.0f%%\n", prefs.Confidence*100)
	fmt.Printf("📈 Muestras totales: %d\n", styleVector[StyleDimensions[0]].Samples)

	// Active patterns
	var patternCount int
	db.QueryRow("SELECT COUNT(*) FROM user_patterns").Scan(&patternCount)
	fmt.Printf("🔄 Patrones activos: %d\n", patternCount)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════")

	return nil
}

// extractPatternData extracts features from user input
func extractPatternData(input string) PatternData {
	runes := []rune(input)
	length := float64(len(runes))

	// Count sentences
	sentences := 0
	for _, r := range runes {
		if r == '.' || r == '!' || r == '?' {
			sentences++
		}
	}
	if sentences == 0 {
		sentences = 1
	}

	// Count questions
	questions := 0
	if strings.Contains(input, "?") {
		questions++
	}

	// Calculate complexity (simple heuristic)
	complexity := 0.0
	longWords := 0
	words := strings.Fields(input)
	for _, w := range words {
		if len([]rune(w)) > 6 {
			longWords++
		}
	}
	if len(words) > 0 {
		complexity = float64(longWords) / float64(len(words))
	}

	return PatternData{
		AvgLength:     length / float64(sentences),
		Complexity:    complexity,
		SentenceCount: sentences,
		QuestionRatio: float64(questions) / float64(sentences),
		CommandRatio:  0.0, // Will be enhanced with more analysis
	}
}

// extractStyleFeatures extracts style dimensions from text
func extractStyleFeatures(text string) map[string]float64 {
	runes := []rune(text)
	length := float64(len(runes))
	words := strings.Fields(text)
	wordCount := float64(len(words))

	features := make(map[string]float64)

	// Verbosity: normalized length (0.0 = very short, 1.0 = very long)
	// Using sigmoid-like normalization
	features["verbosity"] = math.Min(length/500.0, 1.0)

	// Formality: based on abbreviations, punctuation, capitalization
	formalIndicators := 0.0
	if strings.Contains(text, "usted") || strings.Contains(text, "Señor") {
		formalIndicators += 0.3
	}
	if !strings.Contains(text, "...") && !strings.Contains(text, "jaja") {
		formalIndicators += 0.2
	}
	// Check for proper capitalization
	if len(runes) > 0 && unicode.IsUpper(runes[0]) {
		formalIndicators += 0.2
	}
	features["formality"] = math.Min(formalIndicators, 1.0)

	// Technicality: jargon detection
	technicalWords := []string{"api", "database", "server", "config", "deploy", "function", "variable", "array", "json", "sql", "http", "tcp", "ip", "url", "git", "docker", "kubernetes", "linux", "bash", "script"}
	technicalCount := 0.0
	lowerText := strings.ToLower(text)
	for _, tw := range technicalWords {
		if strings.Contains(lowerText, tw) {
			technicalCount++
		}
	}
	features["technicality"] = math.Min(technicalCount/3.0, 1.0)

	// Directness: imperative sentences, questions
	directIndicators := 0.0
	if strings.Contains(text, "?") {
		directIndicators += 0.3
	}
	imperativeWords := []string{"haz", "crea", "ejecuta", "guarda", "muestra", "dame", "busca"}
	for _, iw := range imperativeWords {
		if strings.Contains(lowerText, iw) {
			directIndicators += 0.2
		}
	}
	features["directness"] = math.Min(directIndicators, 1.0)

	// Emotionality: emoticons, exclamation marks, emotional words
	emotionalIndicators := 0.0
	if strings.Contains(text, "!") || strings.Contains(text, "¡") {
		emotionalIndicators += 0.2
	}
	emotionalWords := []string{"genial", "increíble", "fantástico", "terrible", "excelente", "perfecto", "mal", "bien", "odio", "amo"}
	for _, ew := range emotionalWords {
		if strings.Contains(lowerText, ew) {
			emotionalIndicators += 0.15
		}
	}
	features["emotionality"] = math.Min(emotionalIndicators, 1.0)

	// Complexity: sentence structure, vocabulary
	if wordCount > 0 {
		avgWordLen := 0.0
		for _, w := range words {
			avgWordLen += float64(len([]rune(w)))
		}
		avgWordLen /= wordCount
		features["complexity"] = math.Min(avgWordLen/6.0, 1.0)
	} else {
		features["complexity"] = 0.5
	}

	// Frequency: will be updated based on session timing (placeholder)
	features["frequency"] = 0.5

	return features
}

// calculateFingerprint creates a simple fingerprint from pattern data
func calculateFingerprint(data PatternData) string {
	// Simple fingerprint based on key ranges
	lengthBucket := int(data.AvgLength / 50) // 0-50, 50-100, etc.
	complexityBucket := int(data.Complexity * 10)
	return fmt.Sprintf("L%d_C%d_S%d", lengthBucket, complexityBucket, data.SentenceCount)
}

// marshalPatternData converts PatternData to JSON string
func marshalPatternData(data PatternData) string {
	b, _ := json.Marshal(data)
	return string(b)
}

// unmarshalPatternData converts JSON string to PatternData
func unmarshalPatternData(s string) PatternData {
	var data PatternData
	json.Unmarshal([]byte(s), &data)
	return data
}

// SessionSummary holds aggregated session statistics
type SessionSummary struct {
	TotalQueries    int
	TotalStores     int
	AvgQueryLength  float64
	AvgComplexity   float64
	QuestionRatio   float64
	TopPatterns     []PatternData
	Timestamp       int64
}

// ConsolidateSessionPreferences analyzes session patterns and updates preferences
func ConsolidateSessionPreferences(db *sql.DB) (*SessionSummary, error) {
	now := nowMs()
	
	// Get recent patterns (from last 24 hours for better consolidation)
	oneDayAgo := now - 86400
	
	rows, err := db.Query(`
		SELECT pattern_type, pattern_data, observation_count, weight
		FROM user_patterns 
		WHERE last_seen > ?
		ORDER BY last_seen DESC
	`, oneDayAgo)
	if err != nil {
		return nil, fmt.Errorf("querying session patterns: %w", err)
	}
	defer rows.Close()
	
	summary := &SessionSummary{
		Timestamp: now,
	}
	
	var totalLength float64
	var totalComplexity float64
	var totalQuestions float64
	var patternCount float64
	
	for rows.Next() {
		var patternType, patternDataJSON string
		var obsCount int
		var weight float64
		
		if err := rows.Scan(&patternType, &patternDataJSON, &obsCount, &weight); err != nil {
			continue
		}
		
		// Count by type
		switch patternType {
		case PatternTypeQuery:
			summary.TotalQueries += obsCount
		case PatternTypeStore:
			summary.TotalStores += obsCount
		}
		
		// Parse pattern data
		data := unmarshalPatternData(patternDataJSON)
		totalLength += data.AvgLength * float64(obsCount)
		totalComplexity += data.Complexity * float64(obsCount)
		totalQuestions += data.QuestionRatio * float64(obsCount)
		patternCount += float64(obsCount)
		
		summary.TopPatterns = append(summary.TopPatterns, data)
	}
	
	// Calculate averages
	if patternCount > 0 {
		summary.AvgQueryLength = totalLength / patternCount
		summary.AvgComplexity = totalComplexity / patternCount
		summary.QuestionRatio = totalQuestions / patternCount
	}
	
	// Update style vectors based on session trends
	updateStyleFromSession(db, summary)
	
	// Update user preferences
	updatePreferencesFromSession(db, summary)
	
	// Store session summary as reflection
	storeSessionSummary(db, summary)
	
	return summary, nil
}

// updateStyleFromSession updates style vectors based on session patterns
func updateStyleFromSession(db *sql.DB, summary *SessionSummary) {
	now := nowMs()
	
	// Update verbosity based on average query length
	if summary.AvgQueryLength > 0 {
		verbosityValue := math.Min(summary.AvgQueryLength/200.0, 1.0) // Normalize
		updateSingleStyle(db, "verbosity", verbosityValue, now)
	}
	
	// Update complexity
	if summary.AvgComplexity > 0 {
		updateSingleStyle(db, "complexity", summary.AvgComplexity, now)
	}
	
	// Update directness based on question ratio
	if summary.QuestionRatio > 0 {
		updateSingleStyle(db, "directness", summary.QuestionRatio, now)
	}
}

// updateSingleStyle updates a single style dimension
func updateSingleStyle(db *sql.DB, dimension string, value float64, now int64) {
	var currentVal float64
	var samples int
	
	err := db.QueryRow(
		"SELECT value, sample_size FROM style_vectors WHERE dimension = ?", dimension,
	).Scan(&currentVal, &samples)
	
	if err == nil {
		// Exponential moving average with session weight
		newSamples := samples + 1
		alpha := 0.3 // Higher weight for session data
		newVal := currentVal*(1-alpha) + value*alpha
		
		db.Exec(
			"UPDATE style_vectors SET value = ?, sample_size = ?, updated_at = ? WHERE dimension = ?",
			newVal, newSamples, now, dimension,
		)
	} else {
		// Insert new
		id := makeID("style")
		db.Exec(
			"INSERT INTO style_vectors (id, dimension, value, sample_size, updated_at) VALUES (?, ?, ?, 1, ?)",
			id, dimension, value, now,
		)
	}
}

// updatePreferencesFromSession updates user preferences based on session
func updatePreferencesFromSession(db *sql.DB, summary *SessionSummary) {
	now := nowMs()
	
	// Determine preferred response length
	var prefLength string
	switch {
	case summary.AvgQueryLength < 50:
		prefLength = "short"
	case summary.AvgQueryLength < 150:
		prefLength = "medium"
	default:
		prefLength = "long"
	}
	
	// Determine preferred detail level
	var prefDetail string
	switch {
	case summary.AvgComplexity < 0.3:
		prefDetail = "brief"
	case summary.AvgComplexity < 0.6:
		prefDetail = "moderate"
	default:
		prefDetail = "comprehensive"
	}
	
	// Determine preferred tone
	var prefTone string
	if summary.QuestionRatio > 0.5 {
		prefTone = "inquisitive"
	} else if summary.AvgComplexity > 0.7 {
		prefTone = "technical"
	} else {
		prefTone = "casual"
	}
	
	// Calculate confidence based on sample count
	totalSamples := summary.TotalQueries + summary.TotalStores
	confidence := math.Min(float64(totalSamples)/10.0, 1.0)
	
	// Store preferences
	prefs := UserPreferences{
		PreferredResponseLength: prefLength,
		PreferredDetailLevel:    prefDetail,
		PreferredTone:           prefTone,
		AlignmentScore:          confidence,
		Confidence:              confidence,
	}
	
	prefsJSON, _ := json.Marshal(prefs)
	
	// Upsert preferences
	_, err := db.Exec(`
		INSERT INTO user_preferences (id, pref_key, pref_value, confidence, updated_at)
		VALUES (?, 'session_preferences', ?, ?, ?)
		ON CONFLICT(pref_key) DO UPDATE SET
			pref_value = ?,
			confidence = ?,
			updated_at = ?
	`, makeID("pref"), string(prefsJSON), confidence, now, string(prefsJSON), confidence, now)
	
	if err != nil {
		fmt.Printf("⚠️ Error guardando preferencias: %v\n", err)
	}
}

// storeSessionSummary stores the session summary as a reflection
func storeSessionSummary(db *sql.DB, summary *SessionSummary) {
	now := nowMs()
	
	// Build summary text
	var parts []string
	parts = append(parts, "=== RESUMEN DE SESIÓN PWS ===")
	parts = append(parts, fmt.Sprintf("Queries: %d | Stores: %d", summary.TotalQueries, summary.TotalStores))
	parts = append(parts, fmt.Sprintf("Longitud promedio: %.0f chars", summary.AvgQueryLength))
	parts = append(parts, fmt.Sprintf("Complejidad: %.2f", summary.AvgComplexity))
	parts = append(parts, fmt.Sprintf("Ratio preguntas: %.2f", summary.QuestionRatio))
	
	// Top patterns
	if len(summary.TopPatterns) > 0 {
		parts = append(parts, "\nPatrones detectados:")
		for i, p := range summary.TopPatterns {
			if i >= 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("  - Longitud: %.0f, Complejidad: %.2f", p.AvgLength, p.Complexity))
		}
	}
	
	summaryText := strings.Join(parts, "\n")
	
	// Create reflection node
	id := makeID("reflection")
	label := "📊 Resumen PWS — sesión consolidada"
	
	db.Exec(
		"INSERT INTO nodes (id, type, label, content, metadata, created_at, updated_at) VALUES (?, 'reflection', ?, ?, '{}', ?, ?)",
		id, label, summaryText, now, now,
	)
	
	// Link to recent episodes
	var recentEp string
	err := db.QueryRow("SELECT id FROM nodes WHERE type = 'episode' ORDER BY created_at DESC LIMIT 1").Scan(&recentEp)
	if err == nil {
		AddEdge(db, id, recentEp, "temporal", 0.8)
	}
}

// ShowSessionSummary displays the current session PWS summary
func ShowSessionSummary(db *sql.DB) error {
	summary, err := ConsolidateSessionPreferences(db)
	if err != nil {
		return err
	}
	
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║     📊 PWS — Resumen de Sesión                  ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
	
	fmt.Printf("🔍 Queries: %d\n", summary.TotalQueries)
	fmt.Printf("💾 Stores: %d\n", summary.TotalStores)
	fmt.Printf("📏 Longitud promedio: %.0f caracteres\n", summary.AvgQueryLength)
	fmt.Printf("🧠 Complejidad: %.2f\n", summary.AvgComplexity)
	fmt.Printf("❓ Ratio preguntas: %.2f\n", summary.QuestionRatio)
	fmt.Println()
	
	// Show updated preferences
	prefs, err := GetUserPreferences(db)
	if err == nil {
		fmt.Println("⚙️  Preferencias actualizadas:")
		fmt.Printf("  Longitud: %s\n", prefs.PreferredResponseLength)
		fmt.Printf("  Detalle: %s\n", prefs.PreferredDetailLevel)
		fmt.Printf("  Tono: %s\n", prefs.PreferredTone)
		fmt.Printf("  Confianza: %.0f%%\n", prefs.Confidence*100)
	}
	
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════")
	
	return nil
}
