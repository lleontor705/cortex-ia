package app

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/targets"
)

func applyHostTarget(verb string, target targets.TargetID, home string, flags runFlags) error {
	opts := targets.ApplyOptions{DryRun: flags.DryRun, Overwrite: flags.Overwrite}
	var (
		res *targets.TargetResult
		err error
	)
	switch target {
	case targets.TargetClaude:
		res, err = targets.InstallClaudeWith(home, opts)
	case targets.TargetAGY:
		res, err = targets.InstallAGYWith(home, opts)
	default:
		return fmt.Errorf("target %s is not a host bundle target", target)
	}
	printHostTarget(target, res)
	if err != nil {
		return fmt.Errorf("%s %s target: %w", verb, targets.TargetLabel(target), err)
	}
	return nil
}

func removeHostTarget(target targets.TargetID, home string, dryRun bool) error {
	var (
		res *targets.TargetResult
		err error
	)
	switch target {
	case targets.TargetClaude:
		res, err = targets.UninstallClaude(home, dryRun)
	case targets.TargetAGY:
		res, err = targets.UninstallAGY(home, dryRun)
	default:
		return fmt.Errorf("target %s is not a host bundle target", target)
	}
	printHostTarget(target, res)
	if err != nil {
		return fmt.Errorf("uninstall %s target: %w", targets.TargetLabel(target), err)
	}
	return nil
}

func printHostTarget(target targets.TargetID, res *targets.TargetResult) {
	if res == nil {
		return
	}
	fmt.Printf("%s target: %s\n", targets.TargetLabel(target), res.Message)
	for _, change := range res.Changes {
		fmt.Printf("  %s\n", change)
	}
}
