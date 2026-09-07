package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: csvdoctor <file.csv>")
		os.Exit(2)
	}

	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csvdoctor: %v\n", err)
		os.Exit(1)
	}

	errs := validate(data)
	if len(errs) == 0 {
		fmt.Printf("%s: ok\n", path)
		return
	}

	lines := bytes.Split(data, []byte("\n"))
	for _, e := range errs {
		fmt.Printf("%s:%d:%d: %s\n", path, e.pos.line, e.pos.col, e.msg)
		printExcerpt(lines, e.pos)
	}

	os.Exit(1)
}

// printExcerpt shows the offending physical line with a caret under the
// column the error was reported at, the way a compiler would.
func printExcerpt(lines [][]byte, pos position) {
	idx := pos.line - 1
	if idx < 0 || idx >= len(lines) {
		return
	}

	text := strings.TrimRight(string(lines[idx]), "\r")
	lineNo := fmt.Sprintf("%d", pos.line)
	gutter := strings.Repeat(" ", len(lineNo))

	fmt.Printf("    %s | %s\n", lineNo, text)
	fmt.Printf("    %s | %s^\n", gutter, strings.Repeat(" ", pos.col-1))
}
