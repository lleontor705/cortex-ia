package tui

// textinput provides cursor-aware single-line text editing helpers used
// across multiple TUI screens (agent builder prompt, backup rename,
// profile create).

// textInsert inserts a string at position pos within text and returns
// the updated text and new cursor position.
func textInsert(text string, pos int, ch string) (string, int) {
	runes := []rune(text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	inserted := []rune(ch)
	out := make([]rune, 0, len(runes)+len(inserted))
	out = append(out, runes[:pos]...)
	out = append(out, inserted...)
	out = append(out, runes[pos:]...)
	return string(out), pos + len(inserted)
}

// textBackspace deletes the character before pos and returns the updated
// text and new cursor position.
func textBackspace(text string, pos int) (string, int) {
	runes := []rune(text)
	if pos <= 0 || len(runes) == 0 {
		return text, 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	out := make([]rune, 0, len(runes)-1)
	out = append(out, runes[:pos-1]...)
	out = append(out, runes[pos:]...)
	return string(out), pos - 1
}

// textDelete deletes the character at pos (forward delete) and returns
// the updated text. The cursor position stays the same.
func textDelete(text string, pos int) string {
	runes := []rune(text)
	if pos < 0 || pos >= len(runes) {
		return text
	}
	out := make([]rune, 0, len(runes)-1)
	out = append(out, runes[:pos]...)
	out = append(out, runes[pos+1:]...)
	return string(out)
}

// textRenderWithCursor returns a display string with an underscore cursor
// inserted at position pos.
func textRenderWithCursor(text string, pos int) string {
	runes := []rune(text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	out := make([]rune, 0, len(runes)+1)
	out = append(out, runes[:pos]...)
	out = append(out, '_')
	out = append(out, runes[pos:]...)
	return string(out)
}

// clampPos clamps a cursor position to valid range [0, len([]rune(text))].
func clampPos(text string, pos int) int {
	n := len([]rune(text))
	if pos < 0 {
		return 0
	}
	if pos > n {
		return n
	}
	return pos
}
