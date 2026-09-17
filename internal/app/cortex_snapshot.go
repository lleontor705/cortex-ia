package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runCortexSnapshot(args []string) error {
	if len(args) == 0 || args[0] != "read" {
		return errors.New("usage: cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]")
	}
	values := make(map[string]string)
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--project" && args[i] != "--id" && args[i] != "--expected-sha256") {
			return errors.New("invalid snapshot read arguments")
		}
		if _, exists := values[args[i]]; exists {
			return errors.New("duplicate snapshot read argument")
		}
		values[args[i]] = args[i+1]
	}
	id, err := strconv.ParseInt(values["--id"], 10, 64)
	if err != nil {
		return errors.New("snapshot observation ID must be an integer")
	}
	snapshots, err := delegation.ReadCortexSnapshots(context.Background(), values["--project"], []delegation.CortexSnapshotRequest{{ID: id, ExpectedSHA256: values["--expected-sha256"]}})
	if err != nil {
		return err
	}
	return printJSON(snapshots[0])
}
