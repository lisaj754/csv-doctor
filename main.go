package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

func main() {
	delimiterFlag := flag.String("delimiter", ",", `field delimiter; a single character, or "\t" for tab`)
	noHeader := flag.Bool("no-header", false, "treat row 1 as data, not a header: use the most common field count as the expected count instead of trusting row 1")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: csvdoctor [--delimiter <char>] [--no-header] <file.csv>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	delimiter, err := parseDelimiter(*delimiterFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csvdoctor: %v\n", err)
		os.Exit(2)
	}

	path := flag.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csvdoctor: %v\n", err)
		os.Exit(1)
	}

	errs := validate(data, delimiter, *noHeader)
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

// parseDelimiter turns the --delimiter flag value into a rune. "\t" is
// accepted literally (as two characters) since a real tab is awkward to
// pass on a command line, and delimiter must not collide with characters
// the scanner already treats as structural.
func parseDelimiter(s string) (rune, error) {
	if s == `\t` {
		return '\t', nil
	}

	r, size := utf8.DecodeRuneInString(s)
	if s == "" || size != len(s) || r == utf8.RuneError {
		return 0, fmt.Errorf("--delimiter must be a single character, got %q", s)
	}
	if r == '\n' || r == '\r' || r == '"' {
		return 0, fmt.Errorf("--delimiter cannot be %q", r)
	}
	return r, nil
}
