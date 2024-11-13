package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ensureTerminalWindow makes sure the application runs in a visible terminal window
func ensureTerminalWindow() {
	// Skip if we're already in a terminal
	if isRunningInTerminal() {
		return
	}

	// Relaunch the application in a terminal if needed
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "start", "cmd", "/k", os.Args[0])
		cmd.Run()
		os.Exit(0)

	case "darwin":
		// For macOS, use Terminal.app with proper path escaping
		executable, err := os.Executable()
		if err != nil {
			log.Printf("Error getting executable path: %v", err)
			return
		}
		// Escape double quotes and backslashes in the path
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
		// Try different terminal emulators in order of preference
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
		// For macOS, check both TERM and parent process
		if os.Getenv("TERM") == "" {
			return false
		}
		// Check if parent process is Terminal.app
		ppid := os.Getppid()
		parentName, err := getProcessName(ppid)
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToLower(parentName), "terminal") ||
			strings.Contains(strings.ToLower(parentName), "iterm")
	default:
		// For Unix-like systems, check if the process has a controlling terminal
		if os.Getenv("TERM") == "dumb" || os.Getenv("TERM") == "" {
			return false
		}

		// Check if stdin, stdout, and stderr are all terminal devices
		for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
			fileInfo, err := f.Stat()
			if err != nil || (fileInfo.Mode()&os.ModeCharDevice) == 0 {
				return false
			}
		}

		// Additional check for GUI environment
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
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}
	// Get the first string before null byte
	for i, b := range cmdline {
		if b == 0 {
			return string(cmdline[:i]), nil
		}
	}
	return string(cmdline), nil
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
