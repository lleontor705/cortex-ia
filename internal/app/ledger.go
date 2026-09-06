package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runLedger(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia ledger <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  fact add <text> [--board <board-id>] [--source <source>]    Record a verified fact in the Task Ledger")
		fmt.Println("  fact list [--board <board-id>] [--json]                     List all verified facts in chronological order")
		fmt.Println("  progress record --summary <text> [--drift] [--action <act>] Record an orchestrator progress evaluation")
		fmt.Println("  status [--board <board-id>] [--json]                        Display full dual ledger report (facts + progress)")
		return nil
	}

	home, err := cortexStateHome()
	if err != nil {
		return err
	}
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	sub := strings.ToLower(args[0])
	switch sub {
	case "fact":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia ledger fact [add|list] [options]")
		}
		op := strings.ToLower(args[1])
		switch op {
		case "add":
			var text, boardID, source string
			boardID = delegation.DefaultBoardID
			source = "orchestrator"

			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--board":
					if i+1 < len(args) {
						boardID = args[i+1]
						i++
					}
				case "--source":
					if i+1 < len(args) {
						source = args[i+1]
						i++
					}
				default:
					if text == "" && !strings.HasPrefix(args[i], "-") {
						text = args[i]
					}
				}
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("fact text is required: cortex-ia ledger fact add <text> [--board <id>]")
			}
			fact, err := store.AddFact(ctx, boardID, text, source)
			if err != nil {
				return err
			}
			fmt.Printf("✅ Recorded fact #%d on board %q: %s\n", fact.ID, fact.BoardID, fact.Fact)
			return nil

		case "list":
			boardID := delegation.DefaultBoardID
			asJSON := false
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--board":
					if i+1 < len(args) {
						boardID = args[i+1]
						i++
					}
				case "--json":
					asJSON = true
				}
			}
			facts, err := store.ListFacts(ctx, boardID)
			if err != nil {
				return err
			}
			if asJSON {
				data, _ := json.MarshalIndent(facts, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			if len(facts) == 0 {
				fmt.Printf("No facts recorded for board %q.\n", boardID)
				return nil
			}
			fmt.Printf("=== Task Ledger (Facts) for board %q ===\n", boardID)
			for _, f := range facts {
				fmt.Printf("[%s] #%d (%s): %s\n", f.CreatedAt[:19], f.ID, f.Source, f.Fact)
			}
			return nil

		default:
			return fmt.Errorf("unknown fact operation: %q (use add or list)", op)
		}

	case "progress":
		if len(args) < 2 || strings.ToLower(args[1]) != "record" {
			return fmt.Errorf("usage: cortex-ia ledger progress record --summary <text> [--cycle <n>] [--drift] [--action <act>]")
		}
		var summary, boardID, action string
		boardID = delegation.DefaultBoardID
		action = "continue"
		drift := false
		cycle := 0

		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--summary":
				if i+1 < len(args) {
					summary = args[i+1]
					i++
				}
			case "--board":
				if i+1 < len(args) {
					boardID = args[i+1]
					i++
				}
			case "--action":
				if i+1 < len(args) {
					action = args[i+1]
					i++
				}
			case "--cycle":
				if i+1 < len(args) {
					cycle, _ = strconv.Atoi(args[i+1])
					i++
				}
			case "--drift":
				drift = true
			}
		}
		if strings.TrimSpace(summary) == "" {
			return fmt.Errorf("progress summary is required: cortex-ia ledger progress record --summary <text>")
		}
		p, err := store.RecordProgress(ctx, boardID, cycle, summary, drift, action)
		if err != nil {
			return err
		}
		driftStr := "No"
		if p.DriftDetected {
			driftStr = "YES (Attention needed)"
		}
		fmt.Printf("✅ Recorded progress cycle #%d on board %q (Drift: %s, Action: %s): %s\n", p.Cycle, p.BoardID, driftStr, p.Action, p.Summary)
		return nil

	case "status":
		boardID := delegation.DefaultBoardID
		asJSON := false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--board":
				if i+1 < len(args) {
					boardID = args[i+1]
					i++
				}
			case "--json":
				asJSON = true
			}
		}
		report, err := store.GetLedger(ctx, boardID)
		if err != nil {
			return err
		}
		if asJSON {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("================================================================\n")
		fmt.Printf(" DUAL LEDGER REPORT — BOARD: %s\n", report.BoardID)
		fmt.Printf("================================================================\n\n")

		fmt.Printf("--- 1. TASK LEDGER (Authoritative Environmental Facts: %d) ---\n", len(report.Facts))
		if len(report.Facts) == 0 {
			fmt.Println("  (No facts recorded yet)")
		} else {
			for _, f := range report.Facts {
				fmt.Printf("  • [#%d | %s] %s\n", f.ID, f.Source, f.Fact)
			}
		}
		fmt.Println()

		fmt.Printf("--- 2. PROGRESS LEDGER (Cycle Evaluations: %d) ---\n", len(report.Progress))
		if len(report.Progress) == 0 {
			fmt.Println("  (No progress cycles recorded yet)")
		} else {
			for _, p := range report.Progress {
				driftIndicator := "✓ OK"
				if p.DriftDetected {
					driftIndicator = "⚠️ DRIFT"
				}
				fmt.Printf("  Cycle %d [%s | Action: %s]: %s\n", p.Cycle, driftIndicator, p.Action, p.Summary)
			}
		}
		fmt.Println()
		return nil

	default:
		return fmt.Errorf("unknown ledger command: %q (use fact, progress, or status)", sub)
	}
}
