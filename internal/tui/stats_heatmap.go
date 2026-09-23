package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

const (
	heatmapRows       = 7
	heatmapMaxLevel   = 4
	heatmapCellWidth  = 2
	heatmapGutter     = 5
	heatmapDateLayout = "2006-01-02"
)

// heatmapGlyphs maps the five intensity levels (0 = no activity) to distinct
// block characters so the gradient survives terminals without color support.
var heatmapGlyphs = [heatmapMaxLevel + 1]string{"·", "░", "▒", "▓", "█"}

// heatmapWeekdays is Sunday-first, matching time.Weekday order.
var heatmapWeekdays = [heatmapRows]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}

var heatmapMonths = [12]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

// heatmapCell is one calendar day placed on the grid; InWindow is false for the
// padding days that complete the first and last week of the range.
type heatmapCell struct {
	Date     string
	Messages int
	Level    int
	InWindow bool
}

// intensityLevel maps a day's message count to one of five levels relative to
// the busiest day in the window; zero activity is always level 0.
func intensityLevel(messages, windowMax int) int {
	if messages <= 0 || windowMax <= 0 {
		return 0
	}
	level := (messages*heatmapMaxLevel + windowMax - 1) / windowMax
	if level < 1 {
		level = 1
	}
	if level > heatmapMaxLevel {
		level = heatmapMaxLevel
	}
	return level
}

// heatmapGrid lays the window's days on Sunday-first week columns of seven
// weekday rows. The range comes from the summary's real FirstDay/LastDay, so no
// fixed-length year grid is ever produced.
func heatmapGrid(days []ocstats.DayBucket, summary ocstats.Summary) [][]heatmapCell {
	first, err := time.ParseInLocation(heatmapDateLayout, summary.FirstDay, time.Local)
	if err != nil {
		return nil
	}
	last, err := time.ParseInLocation(heatmapDateLayout, summary.LastDay, time.Local)
	if err != nil || last.Before(first) {
		return nil
	}

	messages := make(map[string]int, len(days))
	windowMax := 0
	for _, day := range days {
		if day.Date < summary.FirstDay || day.Date > summary.LastDay {
			continue
		}
		messages[day.Date] = day.Messages
		if day.Messages > windowMax {
			windowMax = day.Messages
		}
	}

	gridStart := first.AddDate(0, 0, -int(first.Weekday()))
	var columns [][]heatmapCell
	for week := gridStart; !week.After(last); week = week.AddDate(0, 0, heatmapRows) {
		column := make([]heatmapCell, heatmapRows)
		for row := range column {
			date := week.AddDate(0, 0, row).Format(heatmapDateLayout)
			inWindow := date >= summary.FirstDay && date <= summary.LastDay
			cell := heatmapCell{Date: date, InWindow: inWindow}
			if inWindow {
				cell.Messages = messages[date]
				cell.Level = intensityLevel(cell.Messages, windowMax)
			}
			column[row] = cell
		}
		columns = append(columns, column)
	}
	return columns
}

// renderHeatmap draws the GitHub-style activity grid for one window: one column
// per week, seven weekday rows, a five-level intensity ramp, and a minimal
// legend.
func renderHeatmap(days []ocstats.DayBucket, summary ocstats.Summary) string {
	columns := heatmapGrid(days, summary)
	if len(columns) == 0 {
		return styleDim.Render("Sin actividad registrada en esta ventana.")
	}

	lines := []string{monthLabelRow(columns), ""}
	for row := 0; row < heatmapRows; row++ {
		var line strings.Builder
		line.WriteString(styleDim.Render(heatmapWeekdays[row]))
		line.WriteString("  ")
		for _, column := range columns {
			cell := column[row]
			if !cell.InWindow {
				line.WriteString(" ")
			} else {
				line.WriteString(heatmapCellStyle(cell.Level).Render(heatmapGlyphs[cell.Level]))
			}
			line.WriteString(" ")
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	lines = append(lines, "", heatmapLegend())
	return strings.Join(lines, "\n")
}

// monthLabelRow places a three-letter month label on the column whose week
// contains that month's first day.
func monthLabelRow(columns [][]heatmapCell) string {
	row := []rune(strings.Repeat(" ", heatmapGutter+len(columns)*heatmapCellWidth))
	for i, column := range columns {
		label := ""
		for _, cell := range column {
			if !cell.InWindow {
				continue
			}
			day, err := time.ParseInLocation(heatmapDateLayout, cell.Date, time.Local)
			if err != nil {
				continue
			}
			if day.Day() == 1 {
				label = heatmapMonths[day.Month()-1]
			}
		}
		start := heatmapGutter + i*heatmapCellWidth
		for j, r := range label {
			if start+j >= len(row) {
				break
			}
			row[start+j] = r
		}
	}
	return styleDim.Render(strings.TrimRight(string(row), " "))
}

func heatmapLegend() string {
	var legend strings.Builder
	legend.WriteString(styleDim.Render("Menos "))
	for level := 0; level <= heatmapMaxLevel; level++ {
		legend.WriteString(heatmapCellStyle(level).Render(heatmapGlyphs[level]))
		legend.WriteString(" ")
	}
	legend.WriteString(styleDim.Render("Más"))
	return legend.String()
}

func heatmapCellStyle(level int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(intensityColor(level))
}

// intensityColor ramps the cell foreground from Muted to Success so the grid
// keeps a gradient even where the block glyphs render at similar weight.
func intensityColor(level int) lipgloss.Color {
	if level <= 0 {
		return styles.Muted
	}
	if level >= heatmapMaxLevel {
		return styles.Success
	}
	t := float64(level) / float64(heatmapMaxLevel)
	return lipgloss.Color(blendHex(string(styles.Muted), string(styles.Success), t))
}

func blendHex(from, to string, t float64) string {
	fr, fg, fb, okFrom := parseHexColor(from)
	tr, tg, tb, okTo := parseHexColor(to)
	if !okFrom || !okTo {
		return to
	}
	return fmt.Sprintf("#%02x%02x%02x",
		int(float64(fr)+(float64(tr)-float64(fr))*t),
		int(float64(fg)+(float64(tg)-float64(fg))*t),
		int(float64(fb)+(float64(tb)-float64(fb))*t))
}

func parseHexColor(value string) (int, int, int, bool) {
	if len(value) != 7 || value[0] != '#' {
		return 0, 0, 0, false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(value[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0, false
	}
	return r, g, b, true
}
