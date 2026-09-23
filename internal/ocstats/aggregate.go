package ocstats

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

const dateLayout = "2006-01-02"

// tokenTotalExpr sums every token class OpenCode pre-aggregates per session.
const tokenTotalExpr = "(tokens_input+tokens_output+tokens_reasoning+tokens_cache_read+tokens_cache_write)"

type rawData struct {
	hasSessions   bool
	firstMs       int64
	lastMs        int64
	sessionsByDay map[string]int
	tokensByDay   map[string]int64
	messagesByDay map[string]int
	hoursByDay    map[string]map[int]int
	modelsByDay   map[string]map[string]modelDayTotals
}

type modelDayTotals struct {
	sessions int
	tokens   int64
}

func scanRaw(db *sql.DB) (*rawData, error) {
	raw := &rawData{
		sessionsByDay: map[string]int{},
		tokensByDay:   map[string]int64{},
		messagesByDay: map[string]int{},
		hoursByDay:    map[string]map[int]int{},
		modelsByDay:   map[string]map[string]modelDayTotals{},
	}

	var minMs, maxMs sql.NullInt64
	var sessions int
	if err := db.QueryRow(
		"SELECT MIN(time_created), MAX(time_created), COUNT(*) FROM session_v2",
	).Scan(&minMs, &maxMs, &sessions); err != nil {
		return nil, fmt.Errorf("read session range: %w", err)
	}
	raw.hasSessions = sessions > 0
	raw.firstMs, raw.lastMs = minMs.Int64, maxMs.Int64

	if err := eachRow(db,
		"SELECT date(time_created/1000,'unixepoch','localtime') d, COUNT(*), SUM("+tokenTotalExpr+") FROM session_v2 GROUP BY d",
		func(scan func(...any) error) error {
			var day string
			var count int
			var tokens sql.NullInt64
			if err := scan(&day, &count, &tokens); err != nil {
				return err
			}
			raw.sessionsByDay[day] = count
			raw.tokensByDay[day] = tokens.Int64
			return nil
		}); err != nil {
		return nil, fmt.Errorf("read daily sessions: %w", err)
	}

	if err := eachRow(db,
		"SELECT date(time_created/1000,'unixepoch','localtime') d, COUNT(*) FROM session_message GROUP BY d",
		func(scan func(...any) error) error {
			var day string
			var count int
			if err := scan(&day, &count); err != nil {
				return err
			}
			raw.messagesByDay[day] = count
			return nil
		}); err != nil {
		return nil, fmt.Errorf("read daily messages: %w", err)
	}

	if err := eachRow(db,
		"SELECT date(time_created/1000,'unixepoch','localtime') d, CAST(strftime('%H', time_created/1000, 'unixepoch', 'localtime') AS INTEGER) h, COUNT(*) FROM session_message GROUP BY d, h",
		func(scan func(...any) error) error {
			var day string
			var hour, count int
			if err := scan(&day, &hour, &count); err != nil {
				return err
			}
			buckets := raw.hoursByDay[day]
			if buckets == nil {
				buckets = map[int]int{}
				raw.hoursByDay[day] = buckets
			}
			buckets[hour] = count
			return nil
		}); err != nil {
		return nil, fmt.Errorf("read hourly messages: %w", err)
	}

	if err := eachRow(db,
		"SELECT json_extract(model,'$.id') m, date(time_created/1000,'unixepoch','localtime') d, COUNT(*), SUM("+tokenTotalExpr+") FROM session_v2 WHERE json_valid(model) AND json_extract(model,'$.id') IS NOT NULL GROUP BY m, d",
		func(scan func(...any) error) error {
			var modelID sql.NullString
			var day string
			var count int
			var tokens sql.NullInt64
			if err := scan(&modelID, &day, &count, &tokens); err != nil {
				return err
			}
			if !modelID.Valid || modelID.String == "" {
				return nil
			}
			byDay := raw.modelsByDay[modelID.String]
			if byDay == nil {
				byDay = map[string]modelDayTotals{}
				raw.modelsByDay[modelID.String] = byDay
			}
			byDay[day] = modelDayTotals{sessions: count, tokens: tokens.Int64}
			return nil
		}); err != nil {
		return nil, fmt.Errorf("read model usage: %w", err)
	}

	return raw, nil
}

func eachRow(db *sql.DB, query string, scan func(func(...any) error) error) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

func reduceReport(path string, raw *rawData, now time.Time) Report {
	report := Report{
		DBPath:    path,
		Days:      buildDays(raw),
		Hours:     buildHours(raw),
		Models:    map[Window][]ModelUsage{},
		Summaries: map[Window]Summary{},
	}
	for _, window := range []Window{WindowAll, Window30d, Window7d} {
		start, end := windowBounds(window, raw, now)
		usage := reduceModels(raw, start, end)
		report.Models[window] = usage
		report.Summaries[window] = reduceSummary(window, raw, start, end, usage)
	}
	return report
}

// windowBounds returns the inclusive local calendar range of a window. Trailing
// windows are anchored to today; WindowAll spans the real data range.
func windowBounds(window Window, raw *rawData, now time.Time) (string, string) {
	if days, ok := trailingWindowDays[window]; ok {
		end := now.Format(dateLayout)
		return now.AddDate(0, 0, -(days - 1)).Format(dateLayout), end
	}
	if !raw.hasSessions {
		return "", ""
	}
	return time.UnixMilli(raw.firstMs).Format(dateLayout), time.UnixMilli(raw.lastMs).Format(dateLayout)
}

// inWindow relies on ISO-8601 dates comparing lexicographically.
func inWindow(day, start, end string) bool {
	return start != "" && day >= start && day <= end
}

func buildDays(raw *rawData) []DayBucket {
	days := make([]DayBucket, 0, len(raw.sessionsByDay)+len(raw.messagesByDay))
	seen := map[string]bool{}
	for day, sessions := range raw.sessionsByDay {
		seen[day] = true
		days = append(days, DayBucket{
			Date:     day,
			Sessions: sessions,
			Messages: raw.messagesByDay[day],
			Tokens:   raw.tokensByDay[day],
		})
	}
	for day, messages := range raw.messagesByDay {
		if seen[day] {
			continue
		}
		days = append(days, DayBucket{Date: day, Messages: messages})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	return days
}

func buildHours(raw *rawData) map[string][]HourBucket {
	hours := make(map[string][]HourBucket, len(raw.hoursByDay))
	for day, buckets := range raw.hoursByDay {
		list := make([]HourBucket, 0, len(buckets))
		for hour, count := range buckets {
			list = append(list, HourBucket{Hour: hour, Messages: count})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Hour < list[j].Hour })
		hours[day] = list
	}
	return hours
}

func reduceModels(raw *rawData, start, end string) []ModelUsage {
	type accumulator struct {
		sessions int
		tokens   int64
	}
	byModel := map[string]*accumulator{}
	for modelID, byDay := range raw.modelsByDay {
		for day, totals := range byDay {
			if !inWindow(day, start, end) {
				continue
			}
			entry := byModel[modelID]
			if entry == nil {
				entry = &accumulator{}
				byModel[modelID] = entry
			}
			entry.sessions += totals.sessions
			entry.tokens += totals.tokens
		}
	}

	var total int64
	usage := make([]ModelUsage, 0, len(byModel))
	for modelID, entry := range byModel {
		total += entry.tokens
		usage = append(usage, ModelUsage{ModelID: modelID, Sessions: entry.sessions, Tokens: entry.tokens})
	}
	sort.Slice(usage, func(i, j int) bool {
		if usage[i].Sessions != usage[j].Sessions {
			return usage[i].Sessions > usage[j].Sessions
		}
		if usage[i].Tokens != usage[j].Tokens {
			return usage[i].Tokens > usage[j].Tokens
		}
		return usage[i].ModelID < usage[j].ModelID
	})
	if total > 0 {
		for i := range usage {
			usage[i].SharePct = math.Round(float64(usage[i].Tokens)/float64(total)*1000) / 10
		}
	}
	return usage
}

func reduceSummary(window Window, raw *rawData, start, end string, models []ModelUsage) Summary {
	summary := Summary{Window: window, FirstDay: start, LastDay: end}
	hours := map[int]int{}
	for day, count := range raw.sessionsByDay {
		if !inWindow(day, start, end) {
			continue
		}
		summary.Sessions += count
		summary.Tokens += raw.tokensByDay[day]
		summary.ActiveDays++
	}
	for day, count := range raw.messagesByDay {
		if !inWindow(day, start, end) {
			continue
		}
		summary.Messages += count
	}
	for day, buckets := range raw.hoursByDay {
		if !inWindow(day, start, end) {
			continue
		}
		for hour, count := range buckets {
			hours[hour] += count
		}
	}
	summary.PeakHour, summary.PeakHourMessages = peakHour(hours)
	if len(models) > 0 {
		summary.FavoriteModel = models[0].ModelID
	}
	return summary
}

// peakHour scans hours in ascending order so equal counts resolve to the
// earliest hour.
func peakHour(totals map[int]int) (int, int) {
	hour, count := 0, 0
	for candidate := 0; candidate < 24; candidate++ {
		if totals[candidate] > count {
			hour, count = candidate, totals[candidate]
		}
	}
	return hour, count
}
