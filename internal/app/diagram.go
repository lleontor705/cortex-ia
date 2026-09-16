package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/diagram"
)

func runDiagram(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia diagram <subcommand> [options]")
		fmt.Println("\nSystem Architecture & Diagram Engine (inspired by archify):")
		fmt.Println("  validate <type> <spec.json> [--quality standard|showcase] [--json]  Validate diagram topology")
		fmt.Println("  render <type> <spec.json> [output.html] [--quality standard|showcase] [--json]  Render interactive diagram HTML")
		fmt.Println("  compare <base.json> <head.json> [output.html] [--json]             Compare architecture snapshots")
		fmt.Println("  reach <type> <spec.json> --from <node-id> [--direction upstream|downstream|both] [--json] Trace graph reach")
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "validate":
		return runDiagramValidate(args[1:])
	case "render":
		return runDiagramRender(args[1:])
	case "compare":
		return runDiagramCompare(args[1:])
	case "reach":
		return runDiagramReach(args[1:])
	default:
		return fmt.Errorf("unknown diagram subcommand %q (see 'cortex-ia diagram --help')", args[0])
	}
}

func runDiagramValidate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia diagram validate <type> <spec.json> [--quality standard|showcase] [--json]")
	}

	var diagType string
	var specFile string
	quality := diagram.QualityStandard
	jsonOutput := false

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--quality=showcase", "showcase":
			quality = diagram.QualityShowcase
		case "--quality=standard", "standard":
			quality = diagram.QualityStandard
		default:
			if !strings.HasPrefix(arg, "-") {
				if diagType == "" {
					diagType = arg
				} else if specFile == "" {
					specFile = arg
				}
			}
		}
	}

	if specFile == "" {
		// Single positional argument might be specFile with type embedded
		if diagType != "" && strings.HasSuffix(diagType, ".json") {
			specFile = diagType
		} else {
			return fmt.Errorf("validate requires a spec JSON file")
		}
	}

	res, err := diagram.ValidateFile(specFile, quality)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	if res.Valid {
		fmt.Printf("✅ Diagram Valid: %s (%s) - %d nodes, %d edges\n", res.Title, res.DiagramType, res.NodeCount, res.EdgeCount)
		for _, w := range res.Warnings {
			fmt.Printf("  ⚠️  [%s] %s (%s)\n", w.Code, w.Message, w.Path)
		}
	} else {
		fmt.Printf("❌ Diagram Validation Failed: %s\n", specFile)
		for _, e := range res.Errors {
			fmt.Printf("  ❌ [%s] %s (%s)\n", e.Code, e.Message, e.Path)
		}
		for _, w := range res.Warnings {
			fmt.Printf("  ⚠️  [%s] %s (%s)\n", w.Code, w.Message, w.Path)
		}
		return fmt.Errorf("diagram validation failed with %d error(s)", len(res.Errors))
	}

	return nil
}

func runDiagramRender(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia diagram render <type> <spec.json> [output.html] [--quality standard|showcase] [--json]")
	}

	var diagType string
	var specFile string
	var outputFile string
	quality := diagram.QualityShowcase
	jsonOutput := false

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--quality=showcase":
			quality = diagram.QualityShowcase
		case "--quality=standard":
			quality = diagram.QualityStandard
		default:
			if !strings.HasPrefix(arg, "-") {
				if diagType == "" {
					diagType = arg
				} else if specFile == "" {
					specFile = arg
				} else if outputFile == "" {
					outputFile = arg
				}
			}
		}
	}

	// If the first argument is a JSON file and no 3rd arg was given, shift so diagType defaults to "architecture"
	if strings.HasSuffix(strings.ToLower(diagType), ".json") && outputFile == "" {
		outputFile = specFile
		specFile = diagType
		diagType = "architecture"
	}

	if specFile == "" {
		return fmt.Errorf("render requires an input specification JSON file")
	}

	if outputFile == "" {
		outputFile = strings.TrimSuffix(specFile, ".json") + ".html"
	}

	res, err := diagram.RenderFile(context.Background(), specFile, outputFile, diagram.RenderOptions{
		DiagramType: diagram.DiagramType(diagType),
		Quality:     quality,
	})
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("🎨 Rendered Diagram: %s -> %s (%d bytes via %s)\n", specFile, res.OutputPath, res.ByteCount, res.EngineUsed)
	return nil
}

func runDiagramCompare(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cortex-ia diagram compare <base.json> <head.json> [output.html] [--json]")
	}

	var basePath, headPath, outputHtml string
	jsonOutput := false

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if !strings.HasPrefix(arg, "-") {
				if basePath == "" {
					basePath = arg
				} else if headPath == "" {
					headPath = arg
				} else if outputHtml == "" {
					outputHtml = arg
				}
			}
		}
	}

	report, err := diagram.CompareFiles(basePath, headPath, outputHtml)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Printf("📊 %s\n", report.Summary)
	if len(report.ComponentsAdded) > 0 {
		fmt.Printf("  • Added Components (%d):\n", len(report.ComponentsAdded))
		for _, c := range report.ComponentsAdded {
			fmt.Printf("      + %s (%s): %s\n", c.ID, c.Type, c.Label)
		}
	}
	if len(report.ComponentsRemoved) > 0 {
		fmt.Printf("  • Removed Components (%d):\n", len(report.ComponentsRemoved))
		for _, c := range report.ComponentsRemoved {
			fmt.Printf("      - %s (%s): %s\n", c.ID, c.Type, c.Label)
		}
	}
	if len(report.ComponentsModified) > 0 {
		fmt.Printf("  • Modified Components (%d):\n", len(report.ComponentsModified))
		for _, c := range report.ComponentsModified {
			fmt.Printf("      ~ %s: %s\n", c.ID, strings.Join(c.Changes, ", "))
		}
	}
	if outputHtml != "" {
		fmt.Printf("  • Comparison HTML: %s\n", outputHtml)
	}

	return nil
}

func runDiagramReach(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia diagram reach <type> <spec.json> --from <node-id> [--direction upstream|downstream|both] [--json]")
	}

	var specFile string
	var fromNode string
	direction := diagram.DirectionDownstream
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--from":
			if i+1 < len(args) {
				i++
				fromNode = args[i]
			}
		case strings.HasPrefix(arg, "--from="):
			fromNode = strings.TrimPrefix(arg, "--from=")
		case arg == "--direction":
			if i+1 < len(args) {
				i++
				direction = diagram.ReachDirection(strings.ToLower(args[i]))
			}
		case strings.HasPrefix(arg, "--direction="):
			direction = diagram.ReachDirection(strings.ToLower(strings.TrimPrefix(arg, "--direction=")))
		default:
			if !strings.HasPrefix(arg, "-") {
				if specFile == "" && strings.HasSuffix(arg, ".json") {
					specFile = arg
				} else if specFile != "" && fromNode == "" {
					fromNode = arg
				}
			}
		}
	}

	if specFile == "" || fromNode == "" {
		return fmt.Errorf("reach requires --from <node-id> and a spec JSON file")
	}

	res, err := diagram.ReachFile(specFile, fromNode, direction)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("🔍 %s\n", res.Summary)
	fmt.Printf("  • Reachable Nodes: %v\n", res.ReachableNodes)
	return nil
}
