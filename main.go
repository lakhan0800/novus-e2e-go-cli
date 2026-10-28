// Command logtail follows a log file and prints lines matching a pattern.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"regexp/syntax"
	"time"
	"unicode/utf8"
)

const (
	defaultPattern = "."
	defaultPath    = "/var/log/system.log"
)

func main() {
	pattern := flag.String("match", defaultPattern, "regular expression to match")
	path := flag.String("file", defaultPath, "log file to follow")
	flag.Parse()

	// Derived values only: the raw pattern and file path may be sensitive.
	patternLength := utf8.RuneCountInString(*pattern)
	matchIsDefault := *pattern == defaultPattern
	fileIsDefault := *path == defaultPath

	re, err := regexp.Compile(*pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid pattern:", err)
		// Track that the -match pattern failed to compile (sent before exit).
		errorCode := "unknown"
		var syntaxErr *syntax.Error
		if errors.As(err, &syntaxErr) {
			errorCode = string(syntaxErr.Code)
		}
		pendoTrack(eventMatchPatternInvalid, map[string]any{
			"pattern_length": patternLength,
			"error_code":     errorCode,
			"exit_code":      2,
		})
		os.Exit(2)
	}

	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open file:", err)
		// Track that the requested log file could not be opened (sent before exit).
		errorType := "other"
		switch {
		case errors.Is(err, fs.ErrNotExist):
			errorType = "not_exist"
		case errors.Is(err, fs.ErrPermission):
			errorType = "permission"
		}
		pendoTrack(eventLogFileOpenFailed, map[string]any{
			"file_is_default": fileIsDefault,
			"error_type":      errorType,
			"exit_code":       1,
		})
		os.Exit(1)
	}
	defer file.Close()

	// Track that following started (pattern compiled, file opened). Sent in
	// the background, once, so it does not delay the read loop.
	go pendoTrack(eventLogFollowStarted, map[string]any{
		"file_is_default":  fileIsDefault,
		"match_is_default": matchIsDefault,
		"pattern_length":   patternLength,
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
