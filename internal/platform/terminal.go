package platform

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnsureTerminalWindow makes sure the application runs in a visible terminal window
func EnsureTerminalWindow() {
	if isRunningInTerminal() {
		return
	}

	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "start", "cmd", "/k", os.Args[0])
		cmd.Run()
		os.Exit(0)

	case "darwin":
		executable, err := os.Executable()
		if err != nil {
			log.Printf("Error getting executable path: %v", err)
			return
		}
		escapedPath := strings.ReplaceAll(executable, `\`, `\\`)
		escapedPath = strings.ReplaceAll(escapedPath, `"`, `\"`)

		script := fmt.Sprintf(`tell application "Terminal"
			do script "\"%s\""
			activate
		end tell`, escapedPath)
		cmd := exec.Command("osascript", "-e", script)
		cmd.Run()
		os.Exit(0)

	case "linux":
		terminals := [][]string{
			{"gnome-terminal", "--", os.Args[0]},
			{"konsole", "-e", os.Args[0]},
			{"xfce4-terminal", "-e", os.Args[0]},
			{"mate-terminal", "-e", os.Args[0]},
			{"x-terminal-emulator", "-e", os.Args[0]},
			{"xterm", "-e", os.Args[0]},
		}

		for _, termCmd := range terminals {
			if path, err := exec.LookPath(termCmd[0]); err == nil {
				cmd := exec.Command(path, termCmd[1:]...)
				cmd.Start()
				os.Exit(0)
			}
		}
		log.Printf("Warning: Could not find a suitable terminal emulator")
	}
}

func isRunningInTerminal() bool {
	switch runtime.GOOS {
	case "windows":
		return os.Getenv("PROMPT") != ""
	case "darwin":
		if os.Getenv("TERM") == "" {
			return false
		}
		ppid := os.Getppid()
		parentName, err := getProcessName(ppid)
		if err != nil {
			return false
		}
		parentNameLower := strings.ToLower(parentName)
		return strings.Contains(parentNameLower, "terminal") ||
			strings.Contains(parentNameLower, "iterm") ||
			strings.Contains(parentNameLower, "zsh") ||
			strings.Contains(parentNameLower, "bash") ||
			strings.Contains(parentNameLower, "sh")
	default:
		if os.Getenv("TERM") == "dumb" || os.Getenv("TERM") == "" {
			return false
		}

		for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
			fileInfo, err := f.Stat()
			if err != nil || (fileInfo.Mode()&os.ModeCharDevice) == 0 {
				return false
			}
		}

		if os.Getenv("DISPLAY") != "" && os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == "" {
			ppid := os.Getppid()
			parentName, err := getProcessName(ppid)
			if err == nil && !isTerminalProcess(parentName) {
				return false
			}
		}

		return true
	}
}

func getProcessName(pid int) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	default:
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			return "", err
		}
		for i, b := range cmdline {
			if b == 0 {
				return string(cmdline[:i]), nil
			}
		}
		return string(cmdline), nil
	}
}

func isTerminalProcess(name string) bool {
	terminalProcesses := []string{
		"gnome-terminal", "konsole", "xfce4-terminal",
		"mate-terminal", "xterm", "terminator",
		"urxvt", "rxvt", "termite", "alacritty",
		"kitty", "bash", "zsh", "sh", "fish",
	}

	for _, term := range terminalProcesses {
		if strings.Contains(name, term) {
			return true
		}
	}
	return false
}
