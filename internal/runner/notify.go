package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// Notify sends a native OS notification for a crashed service.
// Silently skips if LOKL_NO_NOTIFICATIONS is set or no notification tool is found.
func Notify(project, service string) {
	if os.Getenv("LOKL_NO_NOTIFICATIONS") != "" {
		return
	}

	title := fmt.Sprintf("lokl: %s crashed", service)
	body := project

	var cmd *exec.Cmd
	if _, err := exec.LookPath("osascript"); err == nil {
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		cmd = exec.Command("osascript", "-e", script)
	} else if _, err := exec.LookPath("notify-send"); err == nil {
		cmd = exec.Command("notify-send", title, body)
	} else {
		return
	}
	_ = cmd.Run()
}
