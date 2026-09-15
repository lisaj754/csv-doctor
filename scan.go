package main

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

type position struct {
	line int
	col  int
}

type csvError struct {
	pos position
	msg string
}

const quoteChar = '"'

// scanner walks a CSV byte slice one rune at a time, keeping track of the
// 1-based line and column of whatever it's about to read. Columns are
// counted in runes, not bytes, so positions stay correct on UTF-8 input.
type scanner struct {
	data      []byte
	i         int
	line      int
	col       int
	delimiter rune
}

func newScanner(data []byte, delimiter rune) *scanner {
	return &scanner{data: data, line: 1, col: 1, delimiter: delimiter}
}

func (s *scanner) pos() position {
	return position{line: s.line, col: s.col}
}

func (s *scanner) eof() bool {
	return s.i >= len(s.data)
}

func (s *scanner) peek() rune {
	if s.eof() {
		return 0
	}
	r, _ := utf8.DecodeRune(s.data[s.i:])
	return r
}

func (s *scanner) next() rune {
	if s.eof() {
		return 0
	}
	r, size := utf8.DecodeRune(s.data[s.i:])
	s.i += size
	if r == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	return r
}

type rowInfo struct {
	fields int
	pos    position
}

// validate scans data as CSV and returns every structural problem found. It
// does not stop at the first error: a malformed file usually has more than
// one thing wrong with it, and a tool that only reports the first makes you
// fix issues one round-trip at a time.
//
// When noHeader is false, row 1 is trusted to set the expected field count,
// as a header normally would. When noHeader is true there is no reason to
// trust row 1 over any other row, so the expected count is instead whichever
// field count occurs most often across the file.
func validate(data []byte, delimiter rune, noHeader bool) []csvError {
	s := newScanner(data, delimiter)
	var errs []csvError
	var rows []rowInfo

	for !s.eof() {
		rowStart := s.pos()
		fields, rowErrs := s.scanRow()
		errs = append(errs, rowErrs...)
		rows = append(rows, rowInfo{fields: fields, pos: rowStart})
	}

	if len(rows) > 0 {
		var expectedFields, expectedFieldsAt int
		if noHeader {
			expectedFields, expectedFieldsAt = modeFieldCount(rows)
		} else {
			expectedFields, expectedFieldsAt = rows[0].fields, 1
		}

		for _, r := range rows {
			if r.fields != expectedFields {
				errs = append(errs, csvError{
					pos: r.pos,
					msg: fmt.Sprintf("row has %d field(s), expected %d (set by row %d)", r.fields, expectedFields, expectedFieldsAt),
				})
			}
		}
	}

	sort.Slice(errs, func(i, j int) bool {
		if errs[i].pos.line != errs[j].pos.line {
			return errs[i].pos.line < errs[j].pos.line
		}
		return errs[i].pos.col < errs[j].pos.col
	})

	return errs
}

// modeFieldCount returns the field count shared by the most rows, and the
// number of the first row with that count. Ties go to whichever qualifying
// count appears earliest in the file.
func modeFieldCount(rows []rowInfo) (count, atRow int) {
	freq := make(map[int]int)
	firstRow := make(map[int]int)
	for i, r := range rows {
		freq[r.fields]++
		if _, seen := firstRow[r.fields]; !seen {
			firstRow[r.fields] = i + 1
		}
	}

	best := rows[0].fields
	for c, f := range freq {
		switch {
		case f > freq[best]:
			best = c
		case f == freq[best] && firstRow[c] < firstRow[best]:
			best = c
		}
	}
	return best, firstRow[best]
}

// scanRow consumes one record, including its terminating newline if there is
// one, and reports how many fields it contained.
func (s *scanner) scanRow() (int, []csvError) {
	var errs []csvError
	fields := 0

	for {
		errs = append(errs, s.scanField()...)
		fields++

		if s.eof() {
			return fields, errs
		}

		switch s.peek() {
		case s.delimiter:
			s.next()
		case '\n':
			s.next()
			return fields, errs
		case '\r':
			s.next()
			if s.peek() == '\n' {
				s.next()
			}
			return fields, errs
		default:
			// scanField only ever stops at delimiter, newline, or EOF, so
			// this is unreachable; bail out rather than loop forever.
			return fields, errs
		}
	}
}

// scanField consumes one field, quoted or bare, and leaves the scanner
// positioned at the delimiter, newline, or EOF that follows it.
func (s *scanner) scanField() []csvError {
	if s.peek() == quoteChar {
		return s.scanQuotedField()
	}
	return s.scanBareField()
}

func (s *scanner) scanBareField() []csvError {
	var errs []csvError
	for !s.eof() {
		switch s.peek() {
		case s.delimiter, '\n', '\r':
			return errs
		case quoteChar:
			errs = append(errs, csvError{
				pos: s.pos(),
				msg: `quote character inside an unquoted field; wrap the whole field in quotes if it needs to contain one`,
			})
			s.next()
		default:
			s.next()
		}
	}
	return errs
}

func (s *scanner) scanQuotedField() []csvError {
	var errs []csvError
	start := s.pos()
	s.next() // consume opening quote

	for {
		if s.eof() {
			errs = append(errs, csvError{
				pos: start,
				msg: "quoted field is never closed (opening quote here has no matching closing quote before end of file)",
			})
			return errs
		}

		if s.peek() == quoteChar {
			s.next()
			if s.peek() == quoteChar {
				// Escaped quote ("") inside the field; consume both and continue.
				s.next()
				continue
			}
			return append(errs, s.scanTrailingGarbage()...)
		}
		s.next()
	}
}

// scanTrailingGarbage reports and consumes any characters between a quoted
// field's closing quote and the next delimiter or newline, e.g. the "def" in
// "abc"def,next
func (s *scanner) scanTrailingGarbage() []csvError {
	if s.eof() {
		return nil
	}
	switch s.peek() {
	case s.delimiter, '\n', '\r':
		return nil
	}

	pos := s.pos()
	for !s.eof() {
		switch s.peek() {
		case s.delimiter, '\n', '\r':
			return []csvError{{pos: pos, msg: `unexpected data after closing quote (did you forget to escape a quote as ""?)`}}
		}
		s.next()
	}
	return []csvError{{pos: pos, msg: `unexpected data after closing quote (did you forget to escape a quote as ""?)`}}
}
