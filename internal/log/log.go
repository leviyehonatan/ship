package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var LogPath string
var logFile *os.File
var VerboseOut io.Writer

func Init(cmd string, verbose bool) (io.Writer, error) {
	dir := filepath.Join(stateDir(), "logs")
	os.MkdirAll(dir, 0755)

	name := fmt.Sprintf("%s-%s.log",
		time.Now().Format("2006-01-02-150405"),
		sanitize(cmd))
	path := filepath.Join(dir, name)

	var err error
	logFile, err = os.Create(path)
	if err != nil {
		return nil, err
	}
	LogPath = path

	fmt.Fprintf(logFile, "# ship %s (%s)\n\n", cmd, time.Now().Format(time.RFC3339))

	if verbose {
		VerboseOut = os.Stdout
	}

	return io.MultiWriter(os.Stdout, logFile), nil
}

func Close() {
	if logFile != nil {
		now := time.Now().Format(time.RFC3339)
		fmt.Fprintf(logFile, "\n# finished at %s\n", now)
		logFile.Close()
	}
}

func Verbose(format string, args ...interface{}) {
	if VerboseOut != nil {
		fmt.Fprintf(VerboseOut, format, args...)
	}
	if logFile != nil {
		fmt.Fprintf(logFile, format, args...)
	}
}

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ship")
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}
