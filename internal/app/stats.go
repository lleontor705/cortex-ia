package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
	"github.com/lleontor705/cortex-ia/internal/tui"
)

type statsJSONOutput struct {
	DBPath             string               `json:"db_path"`
	Sessions           int                  `json:"sessions"`
	Messages           int                  `json:"messages"`
	Tokens             int64                `json:"tokens"`
	ActiveDays         int                  `json:"active_days"`
	PeakHour           int                  `json:"peak_hour"`
	PeakHourMessages   int                  `json:"peak_hour_messages"`
	FavoriteModel      string               `json:"favorite_model"`
	FavoriteModelShare float64              `json:"favorite_model_share"`
	FirstDay           string               `json:"first_day"`
	LastDay            string               `json:"last_day"`
	TopModels          []ocstats.ModelUsage `json:"top_models"`
}

func runStats(args []string) error {
	jsonMode := false
	for _, arg := range args {
		if isHelp(arg) {
			fmt.Println("Usage: cortex-ia stats [options]")
			fmt.Println("\nOptions:")
			fmt.Println("  --json    Output aggregated statistics in JSON format")
			fmt.Println("  --help    Show this help message")
			return nil
		}
		if arg == "--json" {
			jsonMode = true
		}
	}

	if jsonMode {
		report, err := ocstats.LoadReport()
		if err != nil {
			return fmt.Errorf("failed to load usage statistics: %w", err)
		}

		s := report.Summaries[ocstats.WindowAll]
		models := report.Models[ocstats.WindowAll]
		var topModels []ocstats.ModelUsage
		var favShare float64
		if len(models) > 0 {
			topModels = models
			if len(topModels) > 5 {
				topModels = topModels[:5]
			}
			favShare = models[0].SharePct
		}

		out := statsJSONOutput{
			DBPath:             report.DBPath,
			Sessions:           s.Sessions,
			Messages:           s.Messages,
			Tokens:             s.Tokens,
			ActiveDays:         s.ActiveDays,
			PeakHour:           s.PeakHour,
			PeakHourMessages:   s.PeakHourMessages,
			FavoriteModel:      s.FavoriteModel,
			FavoriteModelShare: favShare,
			FirstDay:           s.FirstDay,
			LastDay:            s.LastDay,
			TopModels:          topModels,
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	return tui.Run(Version)
}
