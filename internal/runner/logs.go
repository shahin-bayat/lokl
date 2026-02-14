package runner

import (
	"strings"
	"sync"
)

type Logs struct {
	lines   []string
	partial string
	max     int
	mu      sync.Mutex
}

func NewLogs(max int) *Logs {
	return &Logs{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (b *Logs) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + string(p)
	lines := strings.Split(text, "\n")

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

func (b *Logs) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}
