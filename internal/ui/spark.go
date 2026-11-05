package ui

import "math"

var sparkLevels = []rune("▁▂▃▄▅▆▇█")

// Sparkline draws values as a row of ▁▂▃▄▅▆▇█ scaled to the largest value,
// at most width runes long. Longer series are resampled into width bins,
// each drawn at its maximum so a single spike stays visible. The result is
// plain; callers draw it in the Accent color.
func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}
	if len(values) > width {
		bins := make([]float64, width)
		for i := range bins {
			from, to := i*len(values)/width, (i+1)*len(values)/width
			for _, v := range values[from:to] {
				bins[i] = math.Max(bins[i], v)
			}
		}
		values = bins
	}
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	out := make([]rune, len(values))
	for i, v := range values {
		level := 0
		if peak > 0 {
			level = int(math.Ceil(v/peak*float64(len(sparkLevels)))) - 1
		}
		out[i] = sparkLevels[max(0, min(len(sparkLevels)-1, level))]
	}
	return string(out)
}

// BarLen returns how many cells of a width-cell bar value fills when peak
// fills all of them. Any positive value gets at least one cell.
func BarLen(value, peak float64, width int) int {
	if value <= 0 || peak <= 0 || width <= 0 {
		return 0
	}
	n := int(math.Round(value / peak * float64(width)))
	if n < 1 {
		return 1
	}
	if n > width {
		return width
	}
	return n
}
