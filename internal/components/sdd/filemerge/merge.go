// Package filemerge performs pure, deterministic three-way merges for managed
// regions embedded in otherwise user-owned files.
package filemerge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
)

const maxDiffCells = 4_000_000

var (
	ErrInvalidRegion     = errors.New("invalid managed region")
	ErrOverlappingRegion = errors.New("managed regions overlap")
)

// ManagedRegion identifies a region by byte offsets in Current. RecordedBase
// is the exact region stored after the previous successful install; Generated
// is the region proposed by the new bundle.
type ManagedRegion struct {
	SemanticID   string
	Start        int
	End          int
	RecordedBase []byte
	Generated    []byte
}

// Conflict retains the complete three-way evidence. The references are stable
// content-addressed SHA-256 references suitable for diagnostics or sidecars.
type Conflict struct {
	SemanticID      string
	RecordedBase    []byte
	Current         []byte
	Generated       []byte
	RecordedBaseRef string
	CurrentRef      string
	GeneratedRef    string
}

// Result contains either the conflict-free merged content or, when Conflicts
// is non-empty, an unchanged copy of Current. A caller can therefore block an
// asset without risking a partial or silent overwrite.
type Result struct {
	Content   []byte
	Conflicts []Conflict
}

// Merge merges all managed regions in one asset. Region order does not affect
// the result. Bytes outside the declared regions are copied verbatim from
// current. If any region conflicts, the whole asset remains unchanged.
func Merge(current []byte, regions []ManagedRegion) (Result, error) {
	ordered := append([]ManagedRegion(nil), regions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start != ordered[j].Start {
			return ordered[i].Start < ordered[j].Start
		}
		if ordered[i].End != ordered[j].End {
			return ordered[i].End < ordered[j].End
		}
		return ordered[i].SemanticID < ordered[j].SemanticID
	})

	if err := validateRegions(len(current), ordered); err != nil {
		return Result{}, err
	}

	mergedRegions := make([][]byte, len(ordered))
	conflicts := make([]Conflict, 0)
	for i, region := range ordered {
		currentRegion := current[region.Start:region.End]
		merged, conflict := mergeRegion(region.RecordedBase, currentRegion, region.Generated)
		if conflict {
			conflicts = append(conflicts, newConflict(region.SemanticID, region.RecordedBase, currentRegion, region.Generated))
			continue
		}
		mergedRegions[i] = merged
	}

	if len(conflicts) != 0 {
		return Result{Content: bytes.Clone(current), Conflicts: conflicts}, nil
	}

	capacity := len(current)
	for i, region := range ordered {
		capacity += len(mergedRegions[i]) - (region.End - region.Start)
	}
	content := make([]byte, 0, capacity)
	position := 0
	for i, region := range ordered {
		content = append(content, current[position:region.Start]...)
		content = append(content, mergedRegions[i]...)
		position = region.End
	}
	content = append(content, current[position:]...)
	return Result{Content: content}, nil
}

func validateRegions(currentLength int, regions []ManagedRegion) error {
	previousEnd := 0
	for i, region := range regions {
		if region.SemanticID == "" || region.Start < 0 || region.End < region.Start || region.End > currentLength {
			return fmt.Errorf("%w %q: byte range [%d,%d) is outside content length %d", ErrInvalidRegion, region.SemanticID, region.Start, region.End, currentLength)
		}
		if i > 0 && (region.Start < previousEnd || region.Start == previousEnd && region.Start == region.End && regions[i-1].Start == regions[i-1].End) {
			return fmt.Errorf("%w: %q starts at byte %d before %q ends at byte %d", ErrOverlappingRegion, region.SemanticID, region.Start, regions[i-1].SemanticID, previousEnd)
		}
		previousEnd = region.End
	}
	return nil
}

func newConflict(semanticID string, base, current, generated []byte) Conflict {
	return Conflict{
		SemanticID:      semanticID,
		RecordedBase:    bytes.Clone(base),
		Current:         bytes.Clone(current),
		Generated:       bytes.Clone(generated),
		RecordedBaseRef: contentRef(base),
		CurrentRef:      contentRef(current),
		GeneratedRef:    contentRef(generated),
	}
}

func contentRef(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

type hunk struct {
	start       int
	end         int
	replacement []byte
}

func mergeRegion(base, current, generated []byte) ([]byte, bool) {
	switch {
	case bytes.Equal(current, generated):
		return bytes.Clone(current), false
	case bytes.Equal(current, base):
		return bytes.Clone(generated), false
	case bytes.Equal(generated, base):
		return bytes.Clone(current), false
	}

	currentHunks, ok := diff(base, current)
	if !ok {
		return nil, true
	}
	generatedHunks, ok := diff(base, generated)
	if !ok {
		return nil, true
	}
	combined, ok := combine(currentHunks, generatedHunks)
	if !ok {
		return nil, true
	}
	return apply(base, combined), false
}

// diff computes deterministic byte hunks using a longest-common-subsequence
// table. Excessively large ambiguous regions fail closed as conflicts rather
// than consuming unbounded memory or guessing.
func diff(base, changed []byte) ([]hunk, bool) {
	if len(base) != 0 && len(changed) > maxDiffCells/len(base) {
		return nil, false
	}
	width := len(changed) + 1
	dp := make([]int, (len(base)+1)*width)
	for i := len(base) - 1; i >= 0; i-- {
		for j := len(changed) - 1; j >= 0; j-- {
			index := i*width + j
			if base[i] == changed[j] {
				dp[index] = 1 + dp[(i+1)*width+j+1]
			} else if dp[(i+1)*width+j] >= dp[i*width+j+1] {
				dp[index] = dp[(i+1)*width+j]
			} else {
				dp[index] = dp[i*width+j+1]
			}
		}
	}

	hunks := make([]hunk, 0)
	i, j := 0, 0
	active := -1
	replacement := make([]byte, 0)
	flush := func() {
		if active < 0 {
			return
		}
		hunks = append(hunks, hunk{start: active, end: i, replacement: bytes.Clone(replacement)})
		active = -1
		replacement = replacement[:0]
	}
	for i < len(base) || j < len(changed) {
		if i < len(base) && j < len(changed) && base[i] == changed[j] {
			flush()
			i++
			j++
			continue
		}
		if active < 0 {
			active = i
		}
		if j < len(changed) && (i == len(base) || dp[i*width+j+1] > dp[(i+1)*width+j]) {
			replacement = append(replacement, changed[j])
			j++
		} else {
			i++
		}
	}
	flush()
	return hunks, true
}

func combine(left, right []hunk) ([]hunk, bool) {
	combined := make([]hunk, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) || j < len(right) {
		switch {
		case i == len(left):
			combined = append(combined, right[j:]...)
			j = len(right)
		case j == len(right):
			combined = append(combined, left[i:]...)
			i = len(left)
		case sameHunk(left[i], right[j]):
			combined = append(combined, left[i])
			i++
			j++
		case hunkBefore(left[i], right[j]):
			combined = append(combined, left[i])
			i++
		case hunkBefore(right[j], left[i]):
			combined = append(combined, right[j])
			j++
		default:
			return nil, false
		}
	}
	return combined, true
}

func sameHunk(a, b hunk) bool {
	return a.start == b.start && a.end == b.end && bytes.Equal(a.replacement, b.replacement)
}

func hunkBefore(a, b hunk) bool {
	if a.end < b.start {
		return true
	}
	if a.end > b.start {
		return false
	}
	// Two insertions at one base position compete; every other touching pair is
	// adjacent and can be applied in base order.
	return a.start != a.end || b.start != b.end
}

func apply(base []byte, hunks []hunk) []byte {
	content := make([]byte, 0, len(base))
	position := 0
	for _, change := range hunks {
		content = append(content, base[position:change.start]...)
		content = append(content, change.replacement...)
		position = change.end
	}
	return append(content, base[position:]...)
}
