package tui

import (
	"strings"
	"unicode"

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
