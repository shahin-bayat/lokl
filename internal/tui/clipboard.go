package tui

import (
	"fmt"
	"os/exec"
	"strings"
)

func copyToClipboard(lines []string) error {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(plainLog(line))
		b.WriteString("\n")
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("pbcopy"); err == nil {
		cmd = exec.Command("pbcopy")
	} else if _, err := exec.LookPath("xclip"); err == nil {
		cmd = exec.Command("xclip", "-selection", "clipboard")
	} else if _, err := exec.LookPath("xsel"); err == nil {
		cmd = exec.Command("xsel", "--clipboard", "--input")
	} else {
		return fmt.Errorf("no clipboard tool found (install pbcopy, xclip, or xsel)")
	}

	cmd.Stdin = strings.NewReader(b.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard write: %w", err)
	}
	return nil
}
