package filemerge

import (
	"bytes"
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestMergeManagedRegions(t *testing.T) {
	tests := []struct {
		name         string
		current      []byte
		regions      []ManagedRegion
		want         []byte
		wantConflict bool
	}{
		{
			name:    "generated update replaces an unchanged region and preserves outside bytes",
			current: []byte("user-prefix\r\nBEGIN\nold\nEND\nuser-suffix\x00"),
			regions: []ManagedRegion{{
				SemanticID:   "region/rule/testing",
				Start:        len("user-prefix\r\nBEGIN\n"),
				End:          len("user-prefix\r\nBEGIN\nold\n"),
				RecordedBase: []byte("old\n"),
				Generated:    []byte("new\n"),
			}},
			want: []byte("user-prefix\r\nBEGIN\nnew\nEND\nuser-suffix\x00"),
		},
		{
			name:    "different line edits merge without conflict",
			current: []byte("before\none: user\ntwo: base\nafter\n"),
			regions: []ManagedRegion{{
				SemanticID:   "region/config",
				Start:        len("before\n"),
				End:          len("before\none: user\ntwo: base\n"),
				RecordedBase: []byte("one: base\ntwo: base\n"),
				Generated:    []byte("one: base\ntwo: generated\n"),
			}},
			want: []byte("before\none: user\ntwo: generated\nafter\n"),
		},
		{
			name:    "identical concurrent edit is accepted",
			current: []byte("x\nsame\ny\n"),
			regions: []ManagedRegion{{
				SemanticID:   "region/identical",
				Start:        len("x\n"),
				End:          len("x\nsame\n"),
				RecordedBase: []byte("base\n"),
				Generated:    []byte("same\n"),
			}},
			want: []byte("x\nsame\ny\n"),
		},
		{
			name:    "same base lines conflict and block the whole asset",
			current: []byte("outside\nuser\ntail\n"),
			regions: []ManagedRegion{{
				SemanticID:   "region/conflict",
				Start:        len("outside\n"),
				End:          len("outside\nuser\n"),
				RecordedBase: []byte("base\n"),
				Generated:    []byte("generated\n"),
			}},
			want:         []byte("outside\nuser\ntail\n"),
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Merge(tt.current, tt.regions)
			if err != nil {
				t.Fatalf("Merge() error = %v", err)
			}
			if !bytes.Equal(got.Content, tt.want) {
				t.Fatalf("Merge() content = %q, want %q", got.Content, tt.want)
			}
			if (len(got.Conflicts) != 0) != tt.wantConflict {
				t.Fatalf("Merge() conflicts = %+v, wantConflict %v", got.Conflicts, tt.wantConflict)
			}
			if tt.wantConflict {
				conflict := got.Conflicts[0]
				if string(conflict.RecordedBase) != "base\n" || string(conflict.Current) != "user\n" || string(conflict.Generated) != "generated\n" {
					t.Fatalf("conflict did not retain all versions: %+v", conflict)
				}
				if conflict.RecordedBaseRef == "" || conflict.CurrentRef == "" || conflict.GeneratedRef == "" {
					t.Fatalf("conflict references must be populated: %+v", conflict)
				}
			}
		})
	}
}

func TestMergeIsDeterministicAndIndependentOfRegionOrder(t *testing.T) {
	current := []byte("prefix\nA0\nmiddle\nB0\nsuffix\n")
	regions := []ManagedRegion{
		{SemanticID: "region/b", Start: len("prefix\nA0\nmiddle\n"), End: len("prefix\nA0\nmiddle\nB0\n"), RecordedBase: []byte("B0\n"), Generated: []byte("B1\n")},
		{SemanticID: "region/a", Start: len("prefix\n"), End: len("prefix\nA0\n"), RecordedBase: []byte("A0\n"), Generated: []byte("A1\n")},
	}

	first, err := Merge(current, regions)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	second, err := Merge(current, []ManagedRegion{regions[1], regions[0]})
	if err != nil {
		t.Fatalf("Merge(reordered) error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("reordered result differs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestMergePreservesOutsideBytesProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(705))
	for i := 0; i < 500; i++ {
		prefix := make([]byte, rng.Intn(64))
		suffix := make([]byte, rng.Intn(64))
		_, _ = rng.Read(prefix)
		_, _ = rng.Read(suffix)
		base := []byte("managed base\n")
		generated := []byte("managed generated\n")
		current := append(append(append([]byte(nil), prefix...), base...), suffix...)

		result, err := Merge(current, []ManagedRegion{{
			SemanticID: "region/property",
			Start:      len(prefix), End: len(prefix) + len(base),
			RecordedBase: base, Generated: generated,
		}})
		if err != nil {
			t.Fatalf("case %d: Merge() error = %v", i, err)
		}
		if !bytes.HasPrefix(result.Content, prefix) || !bytes.HasSuffix(result.Content, suffix) {
			t.Fatalf("case %d: outside bytes changed", i)
		}
	}
}

func TestMergeCombinesDisjointByteEditsProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(1705))
	for i := 0; i < 500; i++ {
		base := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
		currentPosition := rng.Intn(len(base))
		generatedPosition := rng.Intn(len(base) - 1)
		if generatedPosition >= currentPosition {
			generatedPosition++
		}
		currentRegion := bytes.Clone(base)
		generatedRegion := bytes.Clone(base)
		currentRegion[currentPosition] = 0xfe
		generatedRegion[generatedPosition] = 0xff
		current := append(append([]byte("prefix\x00"), currentRegion...), []byte("\xffsuffix")...)

		result, err := Merge(current, []ManagedRegion{{
			SemanticID:   "region/disjoint-property",
			Start:        len("prefix\x00"),
			End:          len("prefix\x00") + len(currentRegion),
			RecordedBase: base,
			Generated:    generatedRegion,
		}})
		if err != nil {
			t.Fatalf("case %d: Merge() error = %v", i, err)
		}
		if len(result.Conflicts) != 0 {
			t.Fatalf("case %d: disjoint edits conflicted: %+v", i, result.Conflicts)
		}
		wantRegion := bytes.Clone(base)
		wantRegion[currentPosition] = 0xfe
		wantRegion[generatedPosition] = 0xff
		if !bytes.Contains(result.Content, wantRegion) {
			t.Fatalf("case %d: merged content does not contain both edits", i)
		}
	}
}

func TestMergeRejectsInvalidOrOverlappingRegions(t *testing.T) {
	tests := []struct {
		name    string
		regions []ManagedRegion
		want    error
	}{
		{name: "missing semantic identity", regions: []ManagedRegion{{Start: 0, End: 1}}, want: ErrInvalidRegion},
		{name: "range outside current", regions: []ManagedRegion{{SemanticID: "region/outside", Start: 0, End: 4}}, want: ErrInvalidRegion},
		{name: "overlapping ranges", regions: []ManagedRegion{
			{SemanticID: "region/a", Start: 0, End: 2},
			{SemanticID: "region/b", Start: 1, End: 3},
		}, want: ErrOverlappingRegion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Merge([]byte("abc"), tt.regions)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Merge() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func FuzzMergeNeverSilentlyOverwritesConflict(f *testing.F) {
	f.Add([]byte("base\n"), []byte("user\n"), []byte("generated\n"), []byte("prefix\x00"), []byte("\xffsuffix"))
	f.Fuzz(func(t *testing.T, base, user, generated, prefix, suffix []byte) {
		if bytes.Equal(user, base) || bytes.Equal(generated, base) || bytes.Equal(user, generated) {
			t.Skip()
		}
		current := append(append(append([]byte(nil), prefix...), user...), suffix...)
		result, err := Merge(current, []ManagedRegion{{
			SemanticID: "region/fuzz",
			Start:      len(prefix), End: len(prefix) + len(user),
			RecordedBase: base, Generated: generated,
		}})
		if err != nil {
			t.Fatalf("Merge() error = %v", err)
		}
		if len(result.Conflicts) > 0 && !bytes.Equal(result.Content, current) {
			t.Fatalf("conflicting merge changed current content")
		}
	})
}
