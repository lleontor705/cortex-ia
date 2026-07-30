package quality

import (
	"fmt"
	"time"
)

// ActivityBudget bounds applicable quality work. Zero means that a dimension
// is not budgeted; negative values are invalid.
type ActivityBudget struct {
	WallTime     time.Duration
	CPUTime      time.Duration
	MemoryBytes  int64
	Cost         float64
	Tokens       int64
	ToolCalls    int
	Retries      int
	Mutants      int
	Cases        int
	FuzzDuration time.Duration
}

// ActivityUsage records consumption in the same dimensions as ActivityBudget.
type ActivityUsage struct {
	WallTime     time.Duration
	CPUTime      time.Duration
	MemoryBytes  int64
	Cost         float64
	Tokens       int64
	ToolCalls    int
	Retries      int
	Mutants      int
	Cases        int
	FuzzDuration time.Duration
}

func (b ActivityBudget) Validate() error {
	if b.WallTime < 0 || b.CPUTime < 0 || b.MemoryBytes < 0 || b.Cost < 0 ||
		b.Tokens < 0 || b.ToolCalls < 0 || b.Retries < 0 || b.Mutants < 0 ||
		b.Cases < 0 || b.FuzzDuration < 0 {
		return fmt.Errorf("quality budget values cannot be negative")
	}
	return nil
}

func (b ActivityBudget) ExhaustedBy(u ActivityUsage) bool {
	return exceeds(b.WallTime, u.WallTime) || exceeds(b.CPUTime, u.CPUTime) ||
		exceeds(b.MemoryBytes, u.MemoryBytes) || exceeds(b.Cost, u.Cost) ||
		exceeds(b.Tokens, u.Tokens) || exceeds(b.ToolCalls, u.ToolCalls) ||
		exceeds(b.Retries, u.Retries) || exceeds(b.Mutants, u.Mutants) ||
		exceeds(b.Cases, u.Cases) || exceeds(b.FuzzDuration, u.FuzzDuration)
}

func exceeds[T ~int | ~int64 | ~float64](budget, used T) bool {
	return budget > 0 && used > budget
}
