package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

const maxWorkRevisionPlanBytes = 128 * 1024

func runWorkRevise(store *delegation.Store, args []string) error {
	if len(args) > 0 && isHelp(args[0]) {
		return workUsage("revise --plan <file|@stdin>", nil)
	}
	opts, positionals, err := workOptions(args, map[string]bool{"--plan": false})
	if err == nil && len(positionals) != 0 {
		err = fmt.Errorf("unknown work revise argument %q", positionals[0])
	}
	if err != nil {
		return workUsage("revise --plan <file|@stdin>", err)
	}
	plan, err := readWorkRevisionPlan(oneOption(opts, "--plan"))
	if err != nil {
		return err
	}
	item, err := store.ReviseWorkDefinition(context.Background(), plan)
	if err != nil {
		return err
	}
	return printJSON(item)
}

func readWorkRevisionPlan(source string) (delegation.WorkRevisionPlan, error) {
	if strings.TrimSpace(source) == "" {
		return delegation.WorkRevisionPlan{}, errors.New("revision plan is required")
	}
	var reader io.Reader
	var file *os.File
	if source == "@stdin" {
		reader = os.Stdin
	} else {
		var err error
		file, err = os.Open(source)
		if err != nil {
			return delegation.WorkRevisionPlan{}, fmt.Errorf("open revision plan: %w", err)
		}
		defer func() { _ = file.Close() }()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxWorkRevisionPlanBytes+1))
	if err != nil {
		return delegation.WorkRevisionPlan{}, fmt.Errorf("read revision plan: %w", err)
	}
	if len(data) > maxWorkRevisionPlanBytes {
		return delegation.WorkRevisionPlan{}, errors.New("revision plan exceeds 128 KiB")
	}
	plan, err := delegation.DecodeWorkRevisionPlan(bytes.NewReader(data))
	if err != nil {
		return delegation.WorkRevisionPlan{}, fmt.Errorf("decode revision plan: %w", err)
	}
	return plan, nil
}
