package config

// Preset describes a named appearance palette.
type Preset struct{ Key, Label, Primary, Accent string }

// Presets returns the supported palettes in display order.
func Presets() []Preset {
	return []Preset{
		{"blue", "Blue", "#4A9EFF", "#4A9EFF"},
		{"green", "Green", "#3FB950", "#3FB950"},
		{"purple", "Purple", "#A371F7", "#A371F7"},
		{"orange", "Orange", "#FEA62B", "#FEA62B"},
		{"rose", "Rose", "#F85149", "#F85149"},
		{"cyan", "Cyan", "#39C5CF", "#39C5CF"},
		{"ocean", "Ocean", "#4A9EFF", "#39C5CF"},
		{"sunset", "Sunset", "#FEA62B", "#F85149"},
		{"grape", "Grape", "#A371F7", "#EC6CB9"},
		{"forest", "Forest", "#3FB950", "#2DD4BF"},
	}
}
