package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/docconv"
)

func runDoc(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia doc <subcommand> [options]")
		fmt.Println("\nUniversal Document to Markdown converter (inspired by anydoc):")
		fmt.Println("  convert <file> [-o <out.md> --standalone] [--format <format>] [--max-lines <n>] [--ocr hosted|reject] [--json]")
		fmt.Println("  inspect <file> [--json]")
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "convert":
		return runDocConvert(args[1:])
	case "inspect":
		return runDocInspect(args[1:])
	default:
		return fmt.Errorf("unknown doc subcommand %q (see 'cortex-ia doc --help')", args[0])
	}
}

func runDocConvert(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia doc convert <file> [-o <out.md>] [--format <fmt>] [--max-lines <n>] [--json]")
	}

	var filePath string
	var outputPath string
	var format docconv.Format
	var maxLines int
	var ocrMode string
	jsonOutput := false
	standalone := false
	authoritySource := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--standalone":
			standalone = true
		case strings.HasPrefix(arg, "--artifact-authority="):
			authoritySource = strings.TrimPrefix(arg, "--artifact-authority=")
		case arg == "-o" || arg == "--output":
			if i+1 < len(args) {
				i++
				outputPath = args[i]
			}
		case strings.HasPrefix(arg, "--output="):
			outputPath = strings.TrimPrefix(arg, "--output=")
		case arg == "-f" || arg == "--format":
			if i+1 < len(args) {
				i++
				format = docconv.Format(strings.ToLower(args[i]))
			}
		case strings.HasPrefix(arg, "--format="):
			format = docconv.Format(strings.ToLower(strings.TrimPrefix(arg, "--format=")))
		case arg == "--max-lines":
			if i+1 < len(args) {
				i++
				maxLines, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--max-lines="):
			maxLines, _ = strconv.Atoi(strings.TrimPrefix(arg, "--max-lines="))
		case arg == "--ocr":
			if i+1 < len(args) {
				i++
				ocrMode = args[i]
			}
		case strings.HasPrefix(arg, "--ocr="):
			ocrMode = strings.TrimPrefix(arg, "--ocr=")
		default:
			if !strings.HasPrefix(arg, "-") && filePath == "" {
				filePath = arg
			}
		}
	}

	if filePath == "" {
		return fmt.Errorf("doc convert requires an input file path")
	}

	check, closeGuard, err := artifactOutputGuard(outputPath, authoritySource, standalone)
	if err != nil {
		return err
	}
	defer closeGuard()
	res, err := docconv.Convert(context.Background(), docconv.ConvertOptions{
		FilePath:    filePath,
		OutputPath:  outputPath,
		Format:      format,
		MaxLines:    maxLines,
		OCRMode:     ocrMode,
		BeforeWrite: check,
	})
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	if outputPath != "" {
		fmt.Printf("✅ Converted %s (%s) -> %s (%d bytes, %d lines)\n", res.FilePath, res.Format, res.OutputPath, res.ByteCount, res.LineCount)
	} else {
		fmt.Print(res.Markdown)
	}

	return nil
}

// CLI adapters parse intent; the delegation service owns output authorization.
func artifactOutputGuard(outputPath, authoritySource string, standalone bool) (func(string) error, func(), error) {
	noop := func() {}
	if outputPath == "" {
		return nil, noop, nil
	}
	var input io.Reader
	if authoritySource != "" {
		if authoritySource != "@stdin" {
			return nil, noop, fmt.Errorf("artifact authority must arrive over stdin")
		}
		input = os.Stdin
	}
	home, err := cortexStateHome()
	if err != nil {
		return nil, noop, err
	}
	guard, err := delegation.OpenArtifactWriteGuard(delegation.DefaultDBPath(home), input, standalone)
	if err != nil {
		return nil, noop, err
	}
	check := func(target string) error { return guard.Check(context.Background(), target) }
	if err := check(outputPath); err != nil {
		_ = guard.Close()
		return nil, noop, err
	}
	return check, func() { _ = guard.Close() }, nil
}

func runDocInspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia doc inspect <file> [--json]")
	}

	var filePath string
	jsonOutput := false

	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else if !strings.HasPrefix(arg, "-") && filePath == "" {
			filePath = arg
		}
	}

	if filePath == "" {
		return fmt.Errorf("doc inspect requires an input file path")
	}

	res, err := docconv.Inspect(filePath)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("📄 Document Inspection: %s\n", res.FilePath)
	fmt.Printf("  • Format: %s\n", res.Format)
	fmt.Printf("  • Size: %d bytes\n", res.SizeBytes)
	if res.EstimatedUnits > 0 {
		unitLabel := res.UnitType
		if len(unitLabel) > 0 {
			unitLabel = strings.ToUpper(unitLabel[:1]) + unitLabel[1:]
		}
		fmt.Printf("  • %s: %d\n", unitLabel, res.EstimatedUnits)
	}
	if len(res.Headings) > 0 {
		fmt.Printf("  • Sections / Headings:\n")
		for _, h := range res.Headings {
			fmt.Printf("      - %s\n", h)
		}
	}
	fmt.Printf("  • Summary: %s\n", res.Summary)

	return nil
}
