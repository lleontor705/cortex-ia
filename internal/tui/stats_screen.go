package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

const (
	statsCardCols      = 3
	statsCardRows      = 2
	statsCardGap       = 1
	statsMinCardWidth  = 10
	statsDefaultWidth  = 80
	statsModelColWidth = 30

	// gatsbyTokensPerNovel pins the playful equivalence of one novel.
	gatsbyTokensPerNovel = 50000

	statsKeyHints      = "tab/←/→ tabs · 1 Todo · 2 30d · 3 7d · esc volver · q salir"
	statsDegradedHints = "esc volver a Home · q salir"
)

type statsTab int

const (
	statsTabSummary statsTab = iota
	statsTabModels
)

var statsWindows = []struct {
	window ocstats.Window
	label  string
}{
	{ocstats.WindowAll, "Todo"},
	{ocstats.Window30d, "30d"},
	{ocstats.Window7d, "7d"},
}

// statsAction is the screen-level navigation a key press requests. The state
// cannot change the model's active screen, so the wiring layer interprets it.
type statsAction int

const (
	statsActionNone statsAction = iota
	statsActionHome
	statsActionQuit
)

var (
	statsCardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(styles.Muted).Padding(0, 1)
	statsValueStyle  = lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
	statsFooterStyle = lipgloss.NewStyle().Italic(true).Foreground(styles.Warning)
)

// statsState keeps the statistics screen self-contained so it renders and
// reacts to keys without sharing fields with the rest of the model.
type statsState struct {
	report  ocstats.Report
	dbPath  string
	err     error
	loading bool
	tab     statsTab
	window  ocstats.Window
}

func newStatsState() statsState {
	return statsState{loading: true, tab: statsTabSummary, window: ocstats.WindowAll}
}

// statsLoadReport is the isolation seam: tests and boot oracles redirect the
// loader so no verification ever opens the user's real OpenCode database.
var statsLoadReport = ocstats.LoadReport

type statsLoadedMsg struct {
	report ocstats.Report
	dbPath string
	err    error
}

func statsLoadCmd() tea.Cmd {
	return func() tea.Msg {
		report, err := statsLoadReport()
		return statsLoadedMsg{report: report, dbPath: statsResolvedPath(report.DBPath, err), err: err}
	}
}

// statsResolvedPath recovers the path named by a typed load failure so the
// degraded screen can always render the resolved location.
func statsResolvedPath(loaded string, err error) string {
	var notFound *ocstats.NotFoundError
	if errors.As(err, &notFound) {
		return notFound.Path
	}
	var unreadable *ocstats.UnreadableError
	if errors.As(err, &unreadable) {
		return unreadable.Path
	}
	if loaded != "" {
		return loaded
	}
	if resolved, resolveErr := ocstats.DefaultDBPath(); resolveErr == nil {
		return resolved
	}
	return ""
}

func (s statsState) onLoaded(msg statsLoadedMsg) statsState {
	s.loading = false
	s.report = msg.report
	s.dbPath = msg.dbPath
	s.err = msg.err
	return s
}

func (s statsState) update(msg tea.KeyMsg) (statsState, statsAction) {
	switch msg.String() {
	case "ctrl+c", "q":
		return s, statsActionQuit
	case "esc":
		return s, statsActionHome
	case "tab", "left", "right":
		if s.tab == statsTabSummary {
			s.tab = statsTabModels
		} else {
			s.tab = statsTabSummary
		}
	case "1":
		s.window = ocstats.WindowAll
	case "2":
		s.window = ocstats.Window30d
	case "3":
		s.window = ocstats.Window7d
	}
	return s, statsActionNone
}

func (s statsState) summary() ocstats.Summary {
	return s.report.Summaries[s.window]
}

func (s statsState) view(width int) string {
	w := width
	if w <= 0 {
		w = statsDefaultWidth
	}

	switch {
	case s.loading:
		return strings.Join([]string{
			statsTitle(),
			"",
			styleDim.Render(styles.SpinnerChar(0) + " Cargando estadísticas de OpenCode…"),
			"",
			styleDim.Render(statsDegradedHints),
		}, "\n")
	case s.err != nil:
		return strings.Join([]string{
			statsTitle(),
			"",
			s.degradedMessage(),
			"",
			styleDim.Render(statsDegradedHints),
		}, "\n")
	}

	body := []string{statsTitle(), "", tabsRow(s.tab), windowsRow(s.window), ""}
	if s.tab == statsTabModels {
		body = append(body, s.modelsView())
	} else {
		body = append(body, s.cardGrid(w), "", renderHeatmap(s.report.Days, s.summary()), "", s.gatsbyFooter())
	}
	body = append(body, "", styleDim.Render(statsKeyHints))
	return strings.Join(body, "\n")
}

func statsTitle() string {
	return styleSubtitle.Render("Estadísticas de uso")
}

func tabsRow(active statsTab) string {
	labels := [2]string{"Resumen", "Modelos"}
	rendered := make([]string, len(labels))
	for i, label := range labels {
		if statsTab(i) == active {
			rendered[i] = styleSelected.Render("[ " + label + " ]")
			continue
		}
		rendered[i] = styleDim.Render("  " + label + "  ")
	}
	return strings.Join(rendered, " ")
}

func windowsRow(active ocstats.Window) string {
	rendered := make([]string, 0, len(statsWindows))
	for i, entry := range statsWindows {
		label := fmt.Sprintf("%d %s", i+1, entry.label)
		if entry.window == active {
			rendered = append(rendered, styleSelected.Render("[ "+label+" ]"))
			continue
		}
		rendered = append(rendered, styleDim.Render("  "+label+"  "))
	}
	return strings.Join(rendered, " ")
}

func (s statsState) cardGrid(width int) string {
	summary := s.summary()
	labels := [statsCardCols * statsCardRows]string{
		"Sesiones", "Mensajes", "Tokens totales", "Días activos", "Hora pico", "Modelo favorito",
	}
	values := [statsCardCols * statsCardRows]string{
		groupThousands(int64(summary.Sessions)),
		groupThousands(int64(summary.Messages)),
		groupThousands(summary.Tokens),
		groupThousands(int64(summary.ActiveDays)),
		peakHourLabel(summary),
		favoriteModelLabel(summary),
	}

	cellWidth := (width - statsCardCols*4 - (statsCardCols-1)*statsCardGap) / statsCardCols
	if cellWidth < statsMinCardWidth {
		cellWidth = statsMinCardWidth
	}

	rows := make([]string, 0, statsCardRows)
	for r := 0; r < statsCardRows; r++ {
		cells := make([]string, 0, statsCardCols)
		for c := 0; c < statsCardCols; c++ {
			i := r*statsCardCols + c
			cell := statsCardStyle.Width(cellWidth)
			if c < statsCardCols-1 {
				cell = cell.MarginRight(statsCardGap)
			}
			content := styleDim.Render(truncate(labels[i], cellWidth)) + "\n" +
				statsValueStyle.Render(truncate(values[i], cellWidth))
			cells = append(cells, cell.Render(content))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func peakHourLabel(summary ocstats.Summary) string {
	if summary.PeakHourMessages <= 0 {
		return "—"
	}
	return fmt.Sprintf("%02d:00", summary.PeakHour)
}

func favoriteModelLabel(summary ocstats.Summary) string {
	if summary.FavoriteModel == "" {
		return "—"
	}
	return summary.FavoriteModel
}

func (s statsState) modelsView() string {
	models := s.report.Models[s.window]
	if len(models) == 0 {
		return styleDim.Render("Sin modelos registrados en esta ventana.")
	}
	lines := []string{styleSubtitle.Render(fmt.Sprintf(
		"%-*s  %8s  %12s  %7s", statsModelColWidth, "Modelo", "Sesiones", "Tokens", "Share"))}
	for _, model := range models {
		lines = append(lines, fmt.Sprintf("%-*s  %8d  %12s  %6.1f%%",
			statsModelColWidth, truncate(model.ModelID, statsModelColWidth),
			model.Sessions, groupThousands(model.Tokens), model.SharePct))
	}
	return strings.Join(lines, "\n")
}

// gatsbyFooter renders the playful token comparison. Below one novel it reports
// the fraction of a Gatsby, so empty windows never divide by zero or print NaN.
func (s statsState) gatsbyFooter() string {
	tokens := s.summary().Tokens
	if tokens < gatsbyTokensPerNovel {
		return statsFooterStyle.Render(fmt.Sprintf("Un %.1f%% de The Great Gatsby en tokens",
			float64(tokens)/float64(gatsbyTokensPerNovel)*100))
	}
	return statsFooterStyle.Render(fmt.Sprintf("%.1fx más tokens que The Great Gatsby",
		float64(tokens)/float64(gatsbyTokensPerNovel)))
}

func (s statsState) degradedMessage() string {
	var notFound *ocstats.NotFoundError
	if errors.As(s.err, &notFound) {
		return styleFail.Render("No se encontró la base de OpenCode en " + notFound.Path)
	}
	var unreadable *ocstats.UnreadableError
	if errors.As(s.err, &unreadable) {
		return styleFail.Render("No se pudo leer la base de OpenCode en " + unreadable.Path)
	}
	return styleFail.Render("No se pudo leer la base de OpenCode en " + s.dbPath)
}

// groupThousands applies hand-rolled grouped separators so the panel needs no
// new formatting dependency.
func groupThousands(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := strconv.FormatInt(n, 10)
	var out strings.Builder
	out.Grow(len(digits) + len(digits)/3)
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteByte(digits[i])
	}
	return sign + out.String()
}
