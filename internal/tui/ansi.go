package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// plainLog strips all ANSI escapes and control runes. Used where styled
// output is wrong: search matching and clipboard copy.
func plainLog(s string) string {
	s = ansi.Strip(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case !unicode.IsControl(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// displayLog keeps SGR color sequences (ESC[...m) and strips every other
// escape family: CSI non-SGR (cursor moves, screen clears), ESC c hard
// reset, OSC (BEL- and ST-terminated), DCS/SOS/PM/APC, and two-byte
// escapes. Control runes are dropped (tabs expand to 4 spaces). An
// incomplete escape at end of line is dropped without touching the text
// before it. Rationale: tools like tsc --watch emit \x1bc which corrupts
// Bubble Tea's alt-screen cursor accounting — color is safe, the rest is not.
func displayLog(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			n, keep := consumeEscape(s[i:])
			if keep {
				b.WriteString(s[i : i+n])
			}
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\t':
			b.WriteString("    ")
		case !unicode.IsControl(r):
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// consumeEscape reports the byte length of the escape sequence starting at
// s[0] == ESC and whether it is an SGR sequence (the only kind kept).
// Incomplete sequences at end of string consume the remainder, keep=false.
func consumeEscape(s string) (n int, keep bool) {
	if len(s) < 2 {
		return len(s), false
	}
	switch s[1] {
	case '[': // CSI: params/intermediates, then final byte 0x40-0x7e
		sgr := true
		for i := 2; i < len(s); i++ {
			c := s[i]
			if c >= 0x40 && c <= 0x7e {
				return i + 1, sgr && c == 'm'
			}
			if c < 0x30 || c > 0x3b {
				sgr = false // private/intermediate bytes: not plain SGR
			}
		}
		return len(s), false
	case ']': // OSC: terminated by BEL or ESC \
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1, false
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2, false
			}
		}
		return len(s), false
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC: terminated by ESC \
		for i := 2; i < len(s); i++ {
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2, false
			}
		}
		return len(s), false
	default: // intermediates 0x20-0x2f then one final byte, e.g. ESC ( B, ESC c
		i := 1
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
			i++
		}
		if i < len(s) {
			return i + 1, false
		}
		return len(s), false
	}
}
