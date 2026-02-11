package platform

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
)

// OpenInOS opens a file or URL in the OS-specific default application.
func OpenInOS(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux", "android", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "illumos":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin", "ios":
		cmd = exec.Command("open", path)
	default:
		return errors.New(i18n.T("unsupported operating system os", "os", runtime.GOOS))
	}

	return cmd.Start()
}

// FatalError handles fatal errors and logs them.
func FatalError(err error, inFile bool, openFile bool, resultFilePath string) {
	errMsg := fmt.Sprintf("ERROR: %s", err)
	if inFile {
		WriteAndOpenFile(resultFilePath, errMsg, openFile)
	}
	log.Fatal(errMsg)
}

// WriteAndOpenFile writes content to a file and optionally opens it.
func WriteAndOpenFile(resultFilePath, content string, openFile bool) {
	if err := os.WriteFile(resultFilePath, []byte(content), 0644); err != nil {
		log.Fatalf("Can't write result file into %s: %#v", resultFilePath, err)
	}
	if openFile {
		if err := OpenInOS(resultFilePath); err != nil {
			log.Fatalf("Can't open result file %s: %#v", resultFilePath, err)
		}
	}
}
