package main

import (
	"regexp"
	"sort"
	"strings"
)

// Entity represents an extracted entity.
type Entity struct {
	Type  string
	Value string
	Conf  float64
}

// Precompiled regex patterns.
var (
	rePhone = regexp.MustCompile(`(\+?\d{1,3}[\s.-]?\(?\d{2,4}\)?[\s.-]?\d{3,4}[\s.-]?\d{3,4})`)
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	reURL   = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `]+`)
	reDate  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	reZip   = regexp.MustCompile(`\b\d{5}\b`)
	reWord  = regexp.MustCompile(`[a-záéíóúñü]{5,}`)
)

// ExtractEntities extracts structured entities from text.
func ExtractEntities(text string) []Entity {
	var entities []Entity

	// Phone numbers
	for _, m := range rePhone.FindAllString(text, -1) {
		entities = append(entities, Entity{"phone", m, 0.95})
	}

	// Emails
	for _, m := range reEmail.FindAllString(text, -1) {
		entities = append(entities, Entity{"email", m, 0.95})
	}

	// URLs
	for _, m := range reURL.FindAllString(text, -1) {
		entities = append(entities, Entity{"url", m, 0.90})
	}

	// Dates
	for _, m := range reDate.FindAllString(text, -1) {
		entities = append(entities, Entity{"date", m, 0.90})
	}

	// ZIP codes
	for _, m := range reZip.FindAllString(text, -1) {
		entities = append(entities, Entity{"zip", m, 0.85})
	}

	// Keywords: words >=5 chars, frequency >=2
	entities = append(entities, extractKeywords(text)...)

	return entities
}

// ExtractEntitiesQuery extracts entities from a short query string.
// Unlike ExtractEntities, it captures single-occurrence keywords since
// queries are typically short and every word matters.
func ExtractEntitiesQuery(text string) []Entity {
	entities := ExtractEntities(text) // reuse phone, email, etc.

	// Add all words >=4 chars as potential keywords (frequency relaxed)
	lower := strings.ToLower(text)
	words := reWord.FindAllString(lower, -1)
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) >= 4 && !seen[w] {
			seen[w] = true
			entities = append(entities, Entity{"keyword", w, 0.40})
		}
	}
	return entities
}

func extractKeywords(text string) []Entity {
	lower := strings.ToLower(text)
	words := reWord.FindAllString(lower, -1)

	freq := make(map[string]int)
	for _, w := range words {
		freq[w]++
	}

	type kv struct {
		word string
		n    int
	}
	var sorted []kv
	for w, n := range freq {
		if n >= 2 {
			sorted = append(sorted, kv{w, n})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].n > sorted[j].n
	})

	var entities []Entity
	for i, kv := range sorted {
		if i >= 20 {
			break
		}
		entities = append(entities, Entity{"keyword", kv.word, 0.50})
	}
	return entities
}
