package logger

import (
	"fmt"
	"io"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

type writer struct {
	out io.Writer
}

// New creates a logger that writes to the provided writer.
func New(w io.Writer) *writer {
	return &writer{out: w}
}

func (l *writer) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = colorize(msg)
	fmt.Fprint(l.out, msg)
}

func (l *writer) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = strings.ReplaceAll(msg, "✗", colorRed+"✗"+colorReset)
	fmt.Fprint(l.out, msg)
}

func colorize(msg string) string {
	msg = strings.ReplaceAll(msg, "✓", colorGreen+"✓"+colorReset)
	msg = strings.ReplaceAll(msg, "⚠", colorYellow+"⚠"+colorReset)
	return msg
}
