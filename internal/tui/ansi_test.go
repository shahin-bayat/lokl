package tui

import "testing"

func TestDisplayLog(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"keeps basic sgr", "\x1b[32mok\x1b[0m", "\x1b[32mok\x1b[0m"},
		{"keeps truecolor sgr", "\x1b[38;2;255;0;0mred\x1b[0m", "\x1b[38;2;255;0;0mred\x1b[0m"},
		{"keeps 256color sgr", "\x1b[48;5;42mbg\x1b[0m", "\x1b[48;5;42mbg\x1b[0m"},
		{"keeps bold and dim", "\x1b[1mbold\x1b[2mdim\x1b[22m", "\x1b[1mbold\x1b[2mdim\x1b[22m"},
		{"drops hard reset", "\x1bcafter", "after"},
		{"drops screen clear", "\x1b[2Jtext", "text"},
		{"drops cursor home", "\x1b[Htext", "text"},
		{"drops cursor move", "\x1b[10;20Htext", "text"},
		{"drops osc bel terminated", "\x1b]0;title\x07text", "text"},
		{"drops osc st terminated", "\x1b]0;title\x1b\\text", "text"},
		{"drops dcs", "\x1bPq#0\x1b\\text", "text"},
		{"drops apc", "\x1b_payload\x1b\\text", "text"},
		{"drops charset designation", "\x1b(Btext", "text"},
		{"expands tab", "a\tb", "a    b"},
		{"drops cr and backspace", "progress\rdone\x08!", "progressdone!"},
		{"drops c1 control rune", "ab", "ab"},
		{"incomplete csi at eol keeps text", "text\x1b[3", "text"},
		{"incomplete osc at eol keeps text", "text\x1b]0;tit", "text"},
		{"bare esc at eol", "text\x1b", "text"},
		{"stdcopy header bytes dropped", "\x01\x00\x00\x00\x00\x00\x00\x05hello", "hello"},
		{"plain text untouched", "just a log line", "just a log line"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayLog(tt.in); got != tt.want {
				t.Errorf("displayLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPlainLog(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"strips sgr", "\x1b[32mok\x1b[0m", "ok"},
		{"strips hard reset", "\x1bcafter", "after"},
		{"expands tab", "a\tb", "a    b"},
		{"drops control runes", "a\rb\x08c", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainLog(tt.in); got != tt.want {
				t.Errorf("plainLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
