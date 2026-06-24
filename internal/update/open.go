package update

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open opens the downloaded installer with the OS default application: Finder
// mounts a .dmg on macOS, Windows runs / opens a .exe. Returns an error if the
// platform is unsupported or the launch command fails.
func Open(path string) error {
	switch runtime.GOOS {
	case "darwin":
		// `open` reveals/launches the file via Finder/LaunchServices.
		if err := exec.Command("open", path).Start(); err != nil {
			return fmt.Errorf("open installer: %w", err)
		}
		return nil
	case "windows":
		// `start "" <path>` opens with the default handler; the empty title
		// arg prevents the path being consumed as the console-window title.
		if err := exec.Command("cmd", "/c", "start", "", path).Start(); err != nil {
			return fmt.Errorf("open installer: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("open installer: unsupported platform %s", runtime.GOOS)
	}
}
