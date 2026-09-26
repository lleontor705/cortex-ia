package tui

import (
	"strings"
	"unicode"
)

// maskedBullet is the single glyph every buffered rune renders as, so the
// rendered width reveals the token length and nothing else.
const maskedBullet = "•"

// maskedInput is the token field. bubbles/textinput is only an indirect
// dependency and the repo has no masked field today, so this manual rune buffer
// keeps zero new direct dependencies and matches the screen rune handling. The
// raw value leaves the buffer exactly once: as the install service argument.
type maskedInput struct {
	runes []rune
}

// insert appends every printable, non-space rune a key or paste event carries.
// A paste arrives as one KeyRunes event holding the whole rune slice, so typing
// and pasting traverse the same path.
func (m *maskedInput) insert(text string) {
	for _, r := range text {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			continue
		}
		m.runes = append(m.runes, r)
	}
}

func (m *maskedInput) backspace() {
	if len(m.runes) == 0 {
		return
	}
	m.runes = m.runes[:len(m.runes)-1]
}

func (m maskedInput) empty() bool { return len(m.runes) == 0 }

// value returns the raw token. It must reach the service call only and never a
// rendered string.
func (m maskedInput) value() string { return string(m.runes) }

// mask returns one bullet per buffered rune; the width is fixed by the buffer,
// independent of the rune widths inside the secret.
func (m maskedInput) mask() string { return strings.Repeat(maskedBullet, len(m.runes)) }
