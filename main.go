// Command logtail follows a log file and prints lines matching a pattern.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultPattern = "."
	defaultPath    = "/var/log/system.log"
)

// openErrorType classifies an os.Open failure for analytics.
func openErrorType(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "not_found"
	case errors.Is(err, fs.ErrPermission):
		return "permission_denied"
	default:
		return "other"
	}
}

func main() {
	pattern := flag.String("match", defaultPattern, "regular expression to match")
	path := flag.String("file", defaultPath, "log file to follow")
	flag.Parse()

	re, err := regexp.Compile(*pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid pattern:", err)
		// Track that the -match pattern was rejected, before exiting.
		pendoTrack(eventPatternCompileFail, map[string]any{
			"pattern_length": len(*pattern),
			"error_message":  truncate(err.Error(), 200),
			"exit_code":      2,
		})
		os.Exit(2)
	}

	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open file:", err)
		// Track that the -file target could not be opened, before exiting.
		pendoTrack(eventLogFileOpenFailed, map[string]any{
			"is_default_file": *path == defaultPath,
			"error_type":      openErrorType(err),
			"exit_code":       1,
		})
		os.Exit(1)
	}
	defer file.Close()

	// Track that a follow session has started. Sent in the background so it
	// does not delay output; it fires once, before the follow loop.
	go pendoTrack(eventLogFollowStarted, map[string]any{
		"pattern":            truncate(*pattern, 200),
		"is_default_pattern": *pattern == defaultPattern,
		"is_default_file":    *path == defaultPath,
		"file_extension":     strings.ToLower(filepath.Ext(*path)),
	})

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if re.MatchString(line) {
			fmt.Print(line)
		}
	}
}
