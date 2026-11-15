package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

func TestHeatmapTodoGridSpansRealRange(t *testing.T) {
	days := []ocstats.DayBucket{
		{Date: "2026-08-01", Sessions: 1, Messages: 5},
		{Date: "2026-09-22", Sessions: 1, Messages: 10},
	}
	summary := ocstats.Summary{Window: ocstats.WindowAll, FirstDay: "2026-08-01", LastDay: "2026-09-22"}

	grid := heatmapGrid(days, summary)
	if len(grid) != 9 {
		t.Fatalf("columns = %d, want 9 (Sun 2026-07-26 through the week of 2026-09-22)", len(grid))
	}
	if first := grid[0][0]; first.Date != "2026-07-26" || first.InWindow {
		t.Fatalf("first cell = %+v, want padding day 2026-07-26", first)
	}
	if cell := grid[0][6]; cell.Date != "2026-08-01" || !cell.InWindow || cell.Messages != 5 {
		t.Fatalf("first-day cell = %+v, want in-window 2026-08-01 with 5 messages", cell)
	}
	if cell := grid[len(grid)-1][2]; cell.Date != "2026-09-22" || !cell.InWindow {
		t.Fatalf("last-day cell = %+v, want in-window 2026-09-22", cell)
	}
}

func TestHeatmapIntensityScalesWithinWindow(t *testing.T) {
	cases := []struct {
		messages, windowMax, want int
	}{
		{0, 40, 0},
		{0, 0, 0},
		{1, 40, 1},
		{10, 40, 1},
		{20, 40, 2},
		{30, 40, 3},
		{40, 40, 4},
	}
	for _, tc := range cases {
		if got := intensityLevel(tc.messages, tc.windowMax); got != tc.want {
			t.Errorf("intensityLevel(%d, %d) = %d, want %d", tc.messages, tc.windowMax, got, tc.want)
		}
	}
}

func TestHeatmapSevenDayWindowClipsAndPlacesDays(t *testing.T) {
	days := []ocstats.DayBucket{
		{Date: "2026-08-01", Messages: 1000},
		{Date: "2026-09-20", Messages: 40},
		{Date: "2026-09-21", Messages: 25},
		{Date: "2026-09-22", Messages: 15},
		{Date: "2026-09-23", Messages: 5},
		{Date: "2026-09-25", Messages: 1},
		{Date: "2026-09-26", Messages: 1},
	}
	summary := ocstats.Summary{Window: ocstats.Window7d, FirstDay: "2026-09-20", LastDay: "2026-09-26"}

	grid := heatmapGrid(days, summary)
	if len(grid) != 1 {
		t.Fatalf("columns = %d, want a single Sunday-first week column", len(grid))
	}
	wantLevels := []int{4, 3, 2, 1, 0, 1, 1}
	for row, want := range wantLevels {
		cell := grid[0][row]
		if !cell.InWindow {
			t.Fatalf("row %d (%s) outside window", row, cell.Date)
		}
		if cell.Level != want {
			t.Errorf("row %d level = %d, want %d (out-of-window buckets must not raise the max)", row, cell.Level, want)
		}
	}
}

func TestHeatmapTrailingWindowUsesWeekdayRows(t *testing.T) {
	summary := ocstats.Summary{Window: ocstats.Window7d, FirstDay: "2026-09-16", LastDay: "2026-09-22"}

	grid := heatmapGrid(nil, summary)
	if len(grid) != 2 {
		t.Fatalf("columns = %d, want 2 for a Wednesday-to-Tuesday trailing week", len(grid))
	}
	if cell := grid[0][3]; cell.Date != "2026-09-16" || !cell.InWindow {
		t.Fatalf("cell = %+v, want 2026-09-16 on the Wednesday row", cell)
	}
	if cell := grid[1][2]; cell.Date != "2026-09-22" || !cell.InWindow {
		t.Fatalf("cell = %+v, want 2026-09-22 on the Tuesday row", cell)
	}
	inWindow := 0
	for _, column := range grid {
		for _, cell := range column {
			if cell.InWindow {
				inWindow++
			}
		}
	}
	if inWindow != 7 {
		t.Fatalf("in-window cells = %d, want exactly the 7 trailing days", inWindow)
	}
}

func TestHeatmapRendersGlyphsMonthsAndLegend(t *testing.T) {
	days := []ocstats.DayBucket{
		{Date: "2026-08-01", Messages: 40},
		{Date: "2026-08-11", Messages: 10},
		{Date: "2026-08-12", Messages: 20},
		{Date: "2026-08-13", Messages: 30},
	}
	summary := ocstats.Summary{Window: ocstats.WindowAll, FirstDay: "2026-08-01", LastDay: "2026-09-26"}

	lines := strings.Split(renderHeatmap(days, summary), "\n")
	if len(lines) != heatmapRows+4 {
		t.Fatalf("lines = %d, want month row + blank + 7 weekday rows + blank + legend", len(lines))
	}
	if !strings.Contains(lines[0], "ago") || !strings.Contains(lines[0], "sep") {
		t.Errorf("month label row = %q, want both ago and sep", lines[0])
	}
	if !strings.Contains(lines[2], "dom") || !strings.Contains(lines[heatmapRows+1], "sáb") {
		t.Errorf("weekday labels missing: %q / %q", lines[2], lines[heatmapRows+1])
	}
	rowGlyphs := map[int]string{0: "·", 2: "░", 3: "▒", 4: "▓", 6: "█"}
	rowOffset := 2
	for row, glyph := range rowGlyphs {
		if !strings.Contains(lines[rowOffset+row], glyph) {
			t.Errorf("weekday row %d = %q, want glyph %q for its intensity level", row, lines[rowOffset+row], glyph)
		}
	}
	legend := lines[heatmapRows+3]
	if !strings.Contains(legend, "Menos") || !strings.Contains(legend, "Más") {
		t.Errorf("legend = %q, want the minimal intensity legend", legend)
	}
	for _, glyph := range heatmapGlyphs {
		if !strings.Contains(legend, glyph) {
			t.Errorf("legend = %q is missing glyph %q", legend, glyph)
		}
	}
}

func TestHeatmapEmptyWindowRendersNotice(t *testing.T) {
	out := renderHeatmap(nil, ocstats.Summary{Window: ocstats.WindowAll})
	if !strings.Contains(out, "Sin actividad") {
		t.Fatalf("output = %q, want the no-activity notice", out)
	}
	if strings.Contains(out, "█") {
		t.Fatalf("output = %q, want no heatmap cells for an empty window", out)
	}
}

func TestHeatmapIntensityColorRamp(t *testing.T) {
	if got := intensityColor(0); got != styles.Muted {
		t.Errorf("level 0 color = %q, want Muted", got)
	}
	if got := intensityColor(heatmapMaxLevel); got != styles.Success {
		t.Errorf("top level color = %q, want Success", got)
	}
	intermediate := map[lipgloss.Color]bool{}
	for level := 1; level < heatmapMaxLevel; level++ {
		intermediate[intensityColor(level)] = true
	}
	if len(intermediate) != heatmapMaxLevel-1 {
		t.Fatalf("intermediate colors = %d distinct, want %d", len(intermediate), heatmapMaxLevel-1)
	}
	if intermediate[styles.Muted] || intermediate[styles.Success] {
		t.Errorf("intermediate colors must stay strictly between Muted and Success")
	}
}
