package process

import (
	"strings"
	"sync"
)

type lineBuffer struct {
	lines   []string
	partial string // incomplete line (no newline yet)
	max     int
	mu      sync.Mutex
}

func newLineBuffer(max int) *lineBuffer {
	return &lineBuffer{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (b *lineBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + string(p)
	lines := strings.Split(text, "\n")

	// Last element is either empty (if ended with \n) or partial line
	b.partial = lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	for _, line := range lines {
		b.lines = append(b.lines, line)
		if len(b.lines) > b.max {
			b.lines = b.lines[1:]
		}
	}

	return len(p), nil
}

func (b *lineBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}
