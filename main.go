// Command logtail follows a log file and prints lines matching a pattern.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"
)

func main() {
	pattern := flag.String("match", ".", "regular expression to match")
	path := flag.String("file", "/var/log/system.log", "log file to follow")
	flag.Parse()

	re, err := regexp.Compile(*pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid pattern:", err)
		os.Exit(2)
	}

	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open file:", err)
		os.Exit(1)
	}
	defer file.Close()

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
