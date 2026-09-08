package tui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Generated from the original Textual Theme for every saved preset and mode.
//
//go:embed theme_values.json
var themeJSON []byte
var themeValues = func() map[string]map[string]string {
	var values map[string]map[string]string
	_ = json.Unmarshal(themeJSON, &values)
	return values
}()

func blendColor(top, bottom string, amount float64) string {
	a, _ := strconv.ParseUint(top[1:], 16, 32)
	b, _ := strconv.ParseUint(bottom[1:], 16, 32)
	result := uint64(0)
	for _, shift := range []uint{16, 8, 0} {
		value := uint64(float64((a>>shift)&255)*amount + float64((b>>shift)&255)*(1-amount))
		result |= value << shift
	}
	return fmt.Sprintf("#%06X", result)
}

func terminalColor(value string) string {
	if _, set := os.LookupEnv("NO_COLOR"); !set {
		return value
	}
	color, _ := strconv.ParseUint(strings.TrimPrefix(value, "#"), 16, 32)
	gray := int(math.Round(float64((color>>16)&255)*.2126 + float64((color>>8)&255)*.7152 + float64(color&255)*.0722))
	return fmt.Sprintf("#%02X%02X%02X", gray, gray, gray)
}
