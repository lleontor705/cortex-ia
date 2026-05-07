package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/gga"
)

// runGGA implements `cortex-ia gga` for the provider switcher.
//
// Flags:
//
//	--provider <id>   switch GGA to <id> (anthropic, openai, google, ollama,
//	                  claude, opencode, gemini, codex)
//	--list            print supported providers and exit
//	--show            print the current ~/.config/gga/config and exit
func runGGA(args []string) error {
	if len(args) == 0 {
		return printGGAUsage()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--list":
			fmt.Println("Supported GGA providers:")
			for _, p := range gga.SupportedProviders {
				fmt.Printf("  - %s\n", p)
			}
			return nil

		case "--show":
			data, readErr := os.ReadFile(gga.ConfigPath(homeDir))
			if readErr != nil {
				if os.IsNotExist(readErr) {
					fmt.Println("(no GGA config installed yet — run `cortex-ia install` first)")
					return nil
				}
				return readErr
			}
			fmt.Println(strings.TrimRight(string(data), "\n"))
			return nil

		case "--provider":
			if i+1 >= len(args) {
				return fmt.Errorf("--provider requires a value")
			}
			i++
			provider := args[i]
			changed, err := gga.SetProvider(homeDir, provider)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("GGA provider set to %s (config: %s)\n", provider, gga.ConfigPath(homeDir))
			} else {
				fmt.Printf("GGA provider already %s — no changes.\n", provider)
			}
			return nil

		case "--help", "-h":
			return printGGAUsage()

		default:
			return fmt.Errorf("gga: unknown flag %q", args[i])
		}
	}

	return printGGAUsage()
}

func printGGAUsage() error {
	fmt.Println("Usage:")
	fmt.Println("  cortex-ia gga --provider <id>     Switch active provider")
	fmt.Println("  cortex-ia gga --list              List supported providers")
	fmt.Println("  cortex-ia gga --show              Print current ~/.config/gga/config")
	return nil
}
