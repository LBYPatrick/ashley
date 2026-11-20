package tui

import (
	_ "embed"
	"os"
	"slices"
	"strings"

	"github.com/LBYPatrick/ashley/internal/config"
)

var languages = []string{"auto", "en", "zh", "ko", "ja", "vi"}
var languageNames = []string{"Terminal", "English", "简体中文", "한국어", "日本語", "Tiếng Việt"}

func resolveLanguage(preference string, getenv func(string) string) string {
	if slices.Contains(languages[1:], preference) {
		return preference
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(getenv(key)); value != "" {
			if lang := localeLanguage(value); lang != "" {
				return lang
			}
			return "en"
		}
	}
	for _, value := range strings.Split(getenv("LANGUAGE"), ":") {
		if lang := localeLanguage(value); lang != "" {
			return lang
		}
	}
	return "en"
}

func localeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.IndexAny(value, "_-.@"); index >= 0 {
		value = value[:index]
	}
	if slices.Contains(languages[1:], value) {
		return value
	}
	return ""
}

//go:embed translations.txt
var translationData string

var translations = loadTranslations()

func loadTranslations() map[string][4]string {
	catalog := make(map[string][4]string)
	for _, line := range strings.Split(strings.TrimSpace(translationData), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 5 {
			catalog[fields[0]] = [4]string{fields[1], fields[2], fields[3], fields[4]}
		}
	}
	return catalog
}

func (m *Model) tr(message string) string {
	if index := slices.Index(languages[2:], m.language); index >= 0 {
		if localized := translations[message][index]; localized != "" {
			return localized
		}
	}
	return message
}

func (m *Model) translatedLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = m.tr(line)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) applyLanguage() {
	m.language = resolveLanguage(m.interfacePrefs.Language, os.Getenv)
	m.composer.Placeholder = m.tr("Describe what you want to build, fix, or understand…")
	m.filter.Placeholder = m.tr("Search…")
	m.question.Placeholder = m.tr("Enter a question, then Enter to run")
}

func (m *Model) saveInterface(value config.InterfacePreferences) {
	if err := m.prefs().SaveInterface(value); err != nil {
		m.status = err.Error()
		return
	}
	m.interfacePrefs = value
	m.applyLanguage()
}
