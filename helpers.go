package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// getAbsolutePath checks if a file exists and returns its absolute path.
func getAbsolutePath(filename string) (string, error) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return filename, fmt.Errorf("error getting absolute path: %v", err)
	}

	_, err = os.Stat(absPath)
	if os.IsNotExist(err) {
		return absPath, fmt.Errorf("file does not exist: %v", absPath)
	} else if err != nil {
		return absPath, fmt.Errorf("error checking file: %v", err)
	}

	return absPath, nil
}

func getFilesByGlob(glob string) ([]string, error) {
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// openFileInOS opens file in OS-specific viewer.
func openFileInOS(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		return err
	}
	return nil
}
