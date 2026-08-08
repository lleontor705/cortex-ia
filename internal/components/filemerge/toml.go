package filemerge

import (
	"fmt"
	"strconv"
	"strings"
)

// TOMLRegionRequest identifies the one MCP server table that a caller has
// independently established is eligible for removal. The locator verifies the
// command vector but deliberately does not make ownership decisions.
type TOMLRegionRequest struct {
	TablePath       []string
	ExpectedCommand string
	ExpectedArgs    []string
}

// TOMLRemovalPlan is a raw-byte splice plan. On a refusal, After is identical
// to Before and SpanStart/SpanEnd are zero.
type TOMLRemovalPlan struct {
	Before    []byte
	After     []byte
	SpanStart int
	SpanEnd   int
	Decision  string
}

type tomlTable struct {
	path       []string
	start, end int
}

// PlanTOMLRegionRemoval locates exactly one canonical MCP server table and
// returns the raw-byte result of removing it. It is intentionally conservative:
// documents with multiline values, duplicate/descendant tables, unusual table
// headers, invalid syntax, or customized table semantics are refused rather
// than normalized or partially rewritten.
func PlanTOMLRegionRemoval(content []byte, request TOMLRegionRequest) (TOMLRemovalPlan, error) {
	plan := TOMLRemovalPlan{Before: append([]byte(nil), content...), After: append([]byte(nil), content...), Decision: "refused"}
	if len(request.TablePath) != 2 || request.TablePath[0] != "mcp_servers" || request.TablePath[1] == "" {
		return plan, fmt.Errorf("TOML removal: request must identify [mcp_servers.<name>]")
	}

	tables, err := scanTOMLTables(content)
	if err != nil {
		return plan, fmt.Errorf("TOML removal: %w", err)
	}

	var target *tomlTable
	for i := range tables {
		table := &tables[i]
		if sameTOMLPath(table.path, request.TablePath) {
			if target != nil {
				return plan, fmt.Errorf("TOML removal: duplicate target table")
			}
			target = table
			continue
		}
		if hasTOMLPathPrefix(table.path, request.TablePath) {
			return plan, fmt.Errorf("TOML removal: target has descendant table")
		}
	}
	if target == nil {
		plan.Decision = "not_found"
		return plan, nil
	}
	if err := validateTOMLTableSemantics(content[target.start:target.end], request); err != nil {
		return plan, fmt.Errorf("TOML removal: %w", err)
	}

	plan.SpanStart, plan.SpanEnd = target.start, target.end
	plan.After = append(append([]byte(nil), content[:target.start]...), content[target.end:]...)
	plan.Decision = "remove"
	return plan, nil
}

func scanTOMLTables(content []byte) ([]tomlTable, error) {
	var tables []tomlTable
	seen := make(map[string]struct{})
	for start := 0; start < len(content); {
		end := len(content)
		if newline := strings.IndexByte(string(content[start:]), '\n'); newline >= 0 {
			end = start + newline
		}
		line := strings.TrimSuffix(string(content[start:end]), "\r")
		trimmed := strings.TrimSpace(line)
		if err := validateTOMLLine(trimmed); err != nil {
			return nil, err
		}
		if strings.HasPrefix(trimmed, "[") {
			path, err := parseTOMLHeader(trimmed)
			if err != nil {
				return nil, err
			}
			key := strings.Join(path, "\x00")
			if _, ok := seen[key]; ok {
				return nil, fmt.Errorf("duplicate table %q", strings.Join(path, "."))
			}
			seen[key] = struct{}{}
			if len(tables) > 0 {
				tables[len(tables)-1].end = start
			}
			tables = append(tables, tomlTable{path: path, start: start, end: len(content)})
		}
		if end == len(content) {
			break
		}
		start = end + 1
	}
	return tables, nil
}

func validateTOMLLine(line string) error {
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if strings.Contains(line, `"""`) || strings.Contains(line, "'''") {
		return fmt.Errorf("multiline values are ambiguous")
	}
	if strings.HasPrefix(line, "[[") {
		return fmt.Errorf("array table headers are ambiguous")
	}
	if strings.HasPrefix(line, "[") {
		_, err := parseTOMLHeader(line)
		return err
	}
	withoutComment, err := stripTOMLComment(line)
	if err != nil {
		return err
	}
	if strings.TrimSpace(withoutComment) == "" {
		return nil
	}
	key, value, found := splitTOMLAssignment(withoutComment)
	if !found {
		return fmt.Errorf("invalid TOML assignment")
	}
	if strings.Contains(key, ".") || strings.Contains(value, "{") || strings.Contains(value, "}") {
		return fmt.Errorf("dotted or inline-table assignment is ambiguous")
	}
	if !balancedTOMLDelimiters(withoutComment) {
		return fmt.Errorf("multiline or unbalanced value is ambiguous")
	}
	return nil
}

func parseTOMLHeader(line string) ([]string, error) {
	withoutComment, err := stripTOMLComment(line)
	if err != nil {
		return nil, err
	}
	withoutComment = strings.TrimSpace(withoutComment)
	if !strings.HasPrefix(withoutComment, "[") || strings.HasPrefix(withoutComment, "[[") {
		return nil, fmt.Errorf("invalid table header")
	}
	close := strings.LastIndex(withoutComment, "]")
	if close < 1 || strings.TrimSpace(withoutComment[close+1:]) != "" {
		return nil, fmt.Errorf("invalid table header")
	}
	return parseTOMLPath(withoutComment[1:close])
}

func parseTOMLPath(value string) ([]string, error) {
	var path []string
	for len(strings.TrimSpace(value)) > 0 {
		value = strings.TrimSpace(value)
		var part string
		if value[0] == '"' {
			end := 1
			for end < len(value) {
				if value[end] == '\\' {
					end += 2
					continue
				}
				if value[end] == '"' {
					break
				}
				end++
			}
			if end >= len(value) || value[end] != '"' {
				return nil, fmt.Errorf("unterminated quoted table key")
			}
			unquoted, err := strconv.Unquote(value[:end+1])
			if err != nil || unquoted == "" {
				return nil, fmt.Errorf("invalid quoted table key")
			}
			part, value = unquoted, value[end+1:]
		} else {
			end := strings.IndexByte(value, '.')
			if end < 0 {
				end = len(value)
			}
			part, value = strings.TrimSpace(value[:end]), value[end:]
			if part == "" || strings.ContainsAny(part, " []\t") {
				return nil, fmt.Errorf("invalid bare table key")
			}
		}
		path = append(path, part)
		value = strings.TrimSpace(value)
		if value == "" {
			break
		}
		if value[0] != '.' {
			return nil, fmt.Errorf("invalid table path separator")
		}
		value = value[1:]
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("empty table path")
	}
	return path, nil
}

func validateTOMLTableSemantics(region []byte, request TOMLRegionRequest) error {
	values := make(map[string]string)
	for _, line := range strings.Split(strings.ReplaceAll(string(region), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		withoutComment, err := stripTOMLComment(trimmed)
		if err != nil {
			return err
		}
		key, value, found := splitTOMLAssignment(withoutComment)
		if !found || (key != "command" && key != "args") {
			return fmt.Errorf("customized or ambiguous target table")
		}
		if _, exists := values[key]; exists {
			return fmt.Errorf("duplicate target key %q", key)
		}
		values[key] = strings.TrimSpace(value)
	}
	command, err := parseTOMLString(values["command"])
	if err != nil || command != request.ExpectedCommand {
		return fmt.Errorf("customized command")
	}
	args, err := parseTOMLStringArray(values["args"])
	if err != nil || len(args) != len(request.ExpectedArgs) {
		return fmt.Errorf("customized args")
	}
	for i := range args {
		if args[i] != request.ExpectedArgs[i] {
			return fmt.Errorf("customized args")
		}
	}
	return nil
}

func stripTOMLComment(value string) (string, error) {
	var quote rune
	escaped := false
	for i, r := range value {
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
		case '#':
			return value[:i], nil
		}
	}
	if quote != 0 || escaped {
		return "", fmt.Errorf("unterminated TOML string")
	}
	return value, nil
}

func splitTOMLAssignment(value string) (string, string, bool) {
	var quote rune
	escaped := false
	for i, r := range value {
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
		case '=':
			key, result := strings.TrimSpace(value[:i]), strings.TrimSpace(value[i+1:])
			if key == "" || result == "" {
				return "", "", false
			}
			return key, result, true
		}
	}
	return "", "", false
}

func balancedTOMLDelimiters(value string) bool {
	var quote rune
	escaped, brackets, braces := false, 0, 0
	for _, r := range value {
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
		case '[':
			brackets++
		case ']':
			brackets--
		case '{':
			braces++
		case '}':
			braces--
		}
		if brackets < 0 || braces < 0 {
			return false
		}
	}
	return quote == 0 && !escaped && brackets == 0 && braces == 0
}

func parseTOMLString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return "", fmt.Errorf("expected TOML string")
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	if value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("expected TOML string")
	}
	decoded, err := strconv.Unquote(value)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func parseTOMLStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, fmt.Errorf("expected one-line TOML string array")
	}
	value = strings.TrimSpace(value[1 : len(value)-1])
	if value == "" {
		return []string{}, nil
	}
	var parts []string
	for len(value) > 0 {
		value = strings.TrimSpace(value)
		if value == "" || (value[0] != '"' && value[0] != '\'') {
			return nil, fmt.Errorf("array contains a non-string value")
		}
		quote := value[0]
		end := 1
		for end < len(value) {
			if quote == '"' && value[end] == '\\' {
				end += 2
				continue
			}
			if value[end] == quote {
				break
			}
			end++
		}
		if end >= len(value) || value[end] != quote {
			return nil, fmt.Errorf("unterminated array string")
		}
		part, err := parseTOMLString(value[:end+1])
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
		value = strings.TrimSpace(value[end+1:])
		if value == "" {
			break
		}
		if value[0] != ',' {
			return nil, fmt.Errorf("array separator missing")
		}
		value = value[1:]
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("trailing array comma is ambiguous")
		}
	}
	return parts, nil
}

func sameTOMLPath(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func hasTOMLPathPrefix(path, prefix []string) bool {
	if len(path) <= len(prefix) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

// UpsertTOMLBlock removes any existing [sectionHeader] block from the given TOML
// content and appends a fresh block with the provided content.
// This is a string-based helper (no TOML parser dependency).
func UpsertTOMLBlock(content, sectionHeader, blockContent string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	headerLine := "[" + sectionHeader + "]"

	var kept []string
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == headerLine {
			i++
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				if strings.HasPrefix(next, "[") && strings.HasSuffix(next, "]") {
					break
				}
				i++
			}
			continue
		}
		kept = append(kept, lines[i])
		i++
	}

	base := strings.TrimSpace(strings.Join(kept, "\n"))
	newBlock := headerLine + "\n" + blockContent

	if base == "" {
		return newBlock + "\n"
	}
	return base + "\n\n" + newBlock + "\n"
}

// UpsertMCPServerTOML is a convenience wrapper for upserting an MCP server block
// in Codex's config.toml format: [mcp_servers.<name>]
func UpsertMCPServerTOML(content, serverName, command string, args []string) string {
	sectionHeader := "mcp_servers." + serverName

	var sb strings.Builder
	fmt.Fprintf(&sb, "command = %q\n", command)
	if len(args) > 0 {
		sb.WriteString("args = [")
		for i, arg := range args {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q", arg)
		}
		sb.WriteString("]\n")
	}

	return UpsertTOMLBlock(content, sectionHeader, sb.String())
}

// UpsertTopLevelTOMLString inserts or replaces a top-level key = "value" pair
// in TOML content. The key is placed before the first [section] header.
func UpsertTopLevelTOMLString(content, key, value string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	lineValue := fmt.Sprintf("%s = %q", key, value)

	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	insertAt := len(cleaned)
	for i, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			insertAt = i
			break
		}
	}

	var out []string
	out = append(out, cleaned[:insertAt]...)
	out = append(out, lineValue)
	out = append(out, cleaned[insertAt:]...)

	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
