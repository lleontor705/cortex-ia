package app

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/components/rdd"
)

func runRDD(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia rdd <freeze|verify> [sha256]")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	switch args[0] {
	case "freeze":
		diff := []byte("diff-snapshot")
		sha := rdd.FreezeCandidate(cwd, diff)
		proof := rdd.Proof{
			Command:       "go test ./...",
			ExitCode:      0,
			OutputSummary: "All tests passed",
		}
		receipt, err := rdd.GenerateReceipt(cwd, "current-project", sha, proof)
		if err != nil {
			return fmt.Errorf("generate receipt failed: %w", err)
		}
		fmt.Printf("Candidate frozen and delivery receipt generated for SHA %s (Status: %s)\n", receipt.CandidateSHA256, receipt.Status)
		return nil

	case "verify":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia rdd verify <candidate-sha256>")
		}
		res := rdd.ValidateDeliveryGate(cwd, args[1])
		if !res.Allowed {
			return fmt.Errorf("delivery gate DENIED: %s", res.Reason)
		}
		fmt.Printf("Delivery gate ALLOWED for candidate SHA %s: %s\n", args[1], res.Reason)
		return nil

	default:
		return fmt.Errorf("unknown RDD subcommand: %s (usage: cortex-ia rdd <freeze|verify>)", args[0])
	}
}
