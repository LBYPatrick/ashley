package tui

import (
	"fmt"
	"math"
	"strconv"
)

// accentGradient matches Textual's CIELAB lightness ramp (-35 to +35).
func accentGradient(accent string, n int) []string {
	shades := make([]string, n)
	for i := range shades {
		shades[i] = accent
		if n > 1 {
			shades[i] = labLightness(accent, (float64(i)/float64(n-1)-.5)*70)
		}
		shades[i] = terminalColor(shades[i])
	}
	return shades
}
func labLightness(hex string, delta float64) string {
	rgb, _ := strconv.ParseUint(hex[1:], 16, 32)
	linear := func(v uint64) float64 {
		c := float64(v&255) / 255
		if c > .04045 {
			return math.Pow((c+.055)/1.055, 2.4)
		}
		return c / 12.92
	}
	r, g, b := linear(rgb>>16), linear(rgb>>8), linear(rgb)
	lab := func(v float64) float64 {
		if v > .008856 {
			return math.Cbrt(v)
		}
		return 7.787*v + 16.0/116
	}
	x := lab((r*41.24 + g*35.76 + b*18.05) / 95.047)
	y := lab((r*21.26 + g*71.52 + b*7.22) / 100)
	z := lab((r*1.93 + g*11.92 + b*95.05) / 108.883)
	light, a, bb := 116*y-16+delta, 500*(x-y), 200*(y-z)
	y = (light + 16) / 116
	x = a/500 + y
	z = y - bb/200
	if y > .2068930344 {
		y = math.Pow(y, 3)
	} else {
		y = (y - 16.0/116) / 7.787
	}
	if x > .2068930344 {
		x = .95047 * math.Pow(x, 3)
	} else {
		x = .122059 * (x - 16.0/116)
	}
	if z > .2068930344 {
		z = 1.08883 * math.Pow(z, 3)
	} else {
		z = .139827 * (z - 16.0/116)
	}
	channel := func(v float64) int {
		if v > .0031308 {
			v = 1.055*math.Pow(v, 1/2.4) - .055
		} else {
			v *= 12.92
		}
		return min(255, max(0, int(v*255)))
	}
	return fmt.Sprintf("#%02X%02X%02X", channel(x*3.2406-y*1.5372-z*.4986), channel(-x*.9689+y*1.8758+z*.0415), channel(x*.0557-y*.2040+z*1.0570))
}
