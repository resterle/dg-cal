package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

//go:embed translations/*.json
var translationsFS embed.FS

// Translator handles internationalization for the web application
type Translator struct {
	translations map[string]map[string]string // lang -> key -> value
	defaultLang  string
	mu           sync.RWMutex
}

// NewTranslator creates a new Translator with translations loaded from embedded files
func NewTranslator(defaultLang string) *Translator {
	t := &Translator{
		translations: make(map[string]map[string]string),
		defaultLang:  defaultLang,
	}
	t.loadTranslations()
	return t
}

// loadTranslations loads all translation files from the embedded filesystem
func (t *Translator) loadTranslations() {
	entries, err := translationsFS.ReadDir("translations")
	if err != nil {
		log.Printf("Failed to read translations directory: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(entry.Name(), ".json")
		data, err := translationsFS.ReadFile("translations/" + entry.Name())
		if err != nil {
			log.Printf("Failed to read translation file %s: %v", entry.Name(), err)
			continue
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			log.Printf("Failed to parse translation file %s: %v", entry.Name(), err)
			continue
		}

		t.mu.Lock()
		t.translations[lang] = translations
		t.mu.Unlock()

		log.Printf("Loaded %d translations for language: %s", len(translations), lang)
	}
}

// T translates a key to the specified language, falling back to default language then key
func (t *Translator) T(lang, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try requested language
	if langMap, ok := t.translations[lang]; ok {
		if val, ok := langMap[key]; ok {
			return val
		}
	}

	// Fall back to default language
	if lang != t.defaultLang {
		if langMap, ok := t.translations[t.defaultLang]; ok {
			if val, ok := langMap[key]; ok {
				return val
			}
		}
	}

	// Return the key itself as fallback (useful during development)
	return key
}

// TWithArgs translates a key and replaces placeholders with provided arguments
// Placeholders use the format {0}, {1}, etc.
func (t *Translator) TWithArgs(lang, key string, args ...interface{}) string {
	translated := t.T(lang, key)

	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		translated = strings.ReplaceAll(translated, placeholder, fmt.Sprint(arg))
	}

	return translated
}

// GetAvailableLanguages returns a list of available language codes
func (t *Translator) GetAvailableLanguages() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	langs := make([]string, 0, len(t.translations))
	for lang := range t.translations {
		langs = append(langs, lang)
	}
	return langs
}

// HasLanguage checks if a language is available
func (t *Translator) HasLanguage(lang string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, ok := t.translations[lang]
	return ok
}

// TemplateFuncs returns template functions for use in html/template
// Usage in templates: {{T "key"}} or {{TArgs "key" arg1 arg2}}
func (t *Translator) TemplateFuncs(lang string) map[string]interface{} {
	return map[string]interface{}{
		"T": func(key string) string {
			return t.T(lang, key)
		},
		"TArgs": func(key string, args ...interface{}) string {
			return t.TWithArgs(lang, key, args...)
		},
	}
}
