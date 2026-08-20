package filemerge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/tailscale/hujson"
)

// JSONMutation describes one atomic object overlay and a set of paths to
// remove before applying it.
type JSONMutation struct {
	Overlay     []byte
	RemovePaths [][]string
}

// JSONFileResult includes the exact bytes surrounding a file mutation so
// callers can produce receipts without reading the file a second time.
type JSONFileResult struct {
	WriteResult
	Before []byte
	After  []byte
}

// DecodeJSONObject parses a strict JSON or JSONC object while rejecting
// duplicate members that would make a targeted mutation ambiguous.
func DecodeJSONObject(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	if err := validateUniqueObjectMembers(raw); err != nil {
		return nil, err
	}
	standard, err := hujson.Standardize(bytes.Clone(raw))
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(standard, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("JSON root must be an object")
	}
	return object, nil
}

// MutateJSONFile validates and applies one JSON or JSONC mutation with a
// single atomic write. JSONC syntax and unaffected comments are preserved.
func MutateJSONFile(path string, mutation JSONMutation) (JSONFileResult, error) {
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return JSONFileResult{}, fmt.Errorf("read JSON file %q: %w", path, err)
	}
	created := os.IsNotExist(err)
	mode := os.FileMode(0o644)
	if !created {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return JSONFileResult{}, fmt.Errorf("stat JSON file %q: %w", path, statErr)
		}
		mode = info.Mode().Perm()
	}

	after, err := MutateJSONDocument(path, before, mutation)
	if err != nil {
		return JSONFileResult{}, fmt.Errorf("mutate JSON file %q: %w", path, err)
	}
	result := JSONFileResult{Before: bytes.Clone(before), After: bytes.Clone(after)}
	if bytes.Equal(before, after) {
		return result, nil
	}
	writeResult, err := WriteFileAtomic(path, after, mode)
	if err != nil {
		return JSONFileResult{}, err
	}
	result.WriteResult = writeResult
	result.Created = created
	return result, nil
}

// MutateJSONDocument applies the same validated transformation as
// MutateJSONFile without filesystem I/O. Both .json and .jsonc are parsed
// with the JWCC parser because OpenCode tolerates trailing commas and
// comments in both extensions; HuJSON Pack() preserves byte-exact output
// for strict JSON input.
func MutateJSONDocument(path string, base []byte, mutation JSONMutation) ([]byte, error) {
	beforeObject, err := DecodeJSONObject(base)
	if err != nil {
		return nil, err
	}
	after, err := mutateJSONC(base, mutation)
	if err != nil {
		return nil, err
	}
	afterObject, err := DecodeJSONObject(after)
	if err != nil {
		return nil, err
	}
	if reflect.DeepEqual(beforeObject, afterObject) {
		return bytes.Clone(base), nil
	}
	return after, nil
}

func mutateJSONC(base []byte, mutation JSONMutation) ([]byte, error) {
	if len(bytes.TrimSpace(base)) == 0 {
		base = []byte("{}\n")
	}
	document, err := hujson.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse JSONC object: %w", err)
	}
	root, ok := document.Value.(*hujson.Object)
	if !ok {
		return nil, fmt.Errorf("JSONC root must be an object")
	}
	if err := validateUniqueHUJSONMembers(root); err != nil {
		return nil, err
	}
	for _, path := range mutation.RemovePaths {
		removeHUJSONPath(root, path)
	}
	if len(bytes.TrimSpace(mutation.Overlay)) > 0 {
		overlayDocument, err := hujson.Parse(mutation.Overlay)
		if err != nil {
			return nil, fmt.Errorf("parse overlay: %w", err)
		}
		if !overlayDocument.IsStandard() {
			return nil, fmt.Errorf("overlay must be strict JSON")
		}
		overlay, ok := overlayDocument.Value.(*hujson.Object)
		if !ok {
			return nil, fmt.Errorf("overlay root must be an object")
		}
		if err := validateUniqueHUJSONMembers(overlay); err != nil {
			return nil, fmt.Errorf("overlay: %w", err)
		}
		mergeHUJSONObjects(root, overlay)
	}
	after := document.Pack()
	if err := validateUniqueObjectMembers(after); err != nil {
		return nil, fmt.Errorf("validate patched JSONC: %w", err)
	}
	return after, nil
}

// replaceSentinel is the reserved overlay key that replaces (instead of
// deep-merges) the object it wraps.
const replaceSentinel = "__replace__"

func mergeHUJSONObjects(base, overlay *hujson.Object) {
	for _, overlayMember := range overlay.Members {
		key := memberName(overlayMember)
		baseIndex := objectMemberIndex(base, key)
		if baseIndex < 0 {
			member := overlayMember
			member.Name.BeforeExtra = inferredMemberIndent(base)
			member.Name.AfterExtra = nil
			member.Value.BeforeExtra = hujson.Extra(" ")
			member.Value.AfterExtra = inheritedTrailingComma(base)
			base.Members = append(base.Members, member)
			continue
		}

		baseValue := &base.Members[baseIndex].Value
		if replacement, ok := hujsonReplacement(overlayMember.Value); ok {
			replaceHUJSONValue(baseValue, replacement)
			continue
		}
		baseObject, baseOK := baseValue.Value.(*hujson.Object)
		overlayObject, overlayOK := overlayMember.Value.Value.(*hujson.Object)
		if baseOK && overlayOK {
			mergeHUJSONObjects(baseObject, overlayObject)
			continue
		}
		replaceHUJSONValue(baseValue, overlayMember.Value)
	}
}

func replaceHUJSONValue(target *hujson.Value, replacement hujson.Value) {
	before, after := target.BeforeExtra, target.AfterExtra
	clone := replacement.Clone()
	*target = clone
	target.BeforeExtra = before
	target.AfterExtra = after
}

func hujsonReplacement(value hujson.Value) (hujson.Value, bool) {
	object, ok := value.Value.(*hujson.Object)
	if !ok || len(object.Members) != 1 || memberName(object.Members[0]) != replaceSentinel {
		return hujson.Value{}, false
	}
	return object.Members[0].Value, true
}

func removeHUJSONPath(object *hujson.Object, path []string) bool {
	if len(path) == 0 {
		return false
	}
	index := objectMemberIndex(object, path[0])
	if index < 0 {
		return false
	}
	if len(path) == 1 {
		object.Members = append(object.Members[:index], object.Members[index+1:]...)
		return true
	}
	child, ok := object.Members[index].Value.Value.(*hujson.Object)
	if !ok || !removeHUJSONPath(child, path[1:]) {
		return false
	}
	if len(child.Members) == 0 {
		object.Members = append(object.Members[:index], object.Members[index+1:]...)
	}
	return true
}

func validateUniqueObjectMembers(raw []byte) error {
	value, err := hujson.Parse(raw)
	if err != nil {
		return err
	}
	root, ok := value.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf("JSON root must be an object")
	}
	return validateUniqueHUJSONMembers(root)
}

func validateUniqueHUJSONMembers(object *hujson.Object) error {
	seen := make(map[string]struct{}, len(object.Members))
	for _, member := range object.Members {
		name := memberName(member)
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate object member %q", name)
		}
		seen[name] = struct{}{}
		if child, ok := member.Value.Value.(*hujson.Object); ok {
			if err := validateUniqueHUJSONMembers(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func memberName(member hujson.ObjectMember) string {
	literal, _ := member.Name.Value.(hujson.Literal)
	return literal.String()
}

func objectMemberIndex(object *hujson.Object, key string) int {
	for index, member := range object.Members {
		if memberName(member) == key {
			return index
		}
	}
	return -1
}

func inferredMemberIndent(object *hujson.Object) hujson.Extra {
	if len(object.Members) == 0 {
		return hujson.Extra("\n  ")
	}
	extra := object.Members[len(object.Members)-1].Name.BeforeExtra
	line := bytes.LastIndexByte(extra, '\n')
	if line >= 0 {
		return bytes.Clone(extra[line:])
	}
	return hujson.Extra(" ")
}

func inheritedTrailingComma(object *hujson.Object) hujson.Extra {
	if len(object.Members) > 0 && object.Members[len(object.Members)-1].Value.AfterExtra != nil {
		return hujson.Extra{}
	}
	return nil
}
