package cppdep

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
)

const (
	QuoteIncludeType = iota
	BracketIncludeType
)

// Scanner is used to scan source files to look for include statements
type Scanner struct {
	scan  *bufio.Scanner
	regex *regexp.Regexp
	text  string
	typ   int
}

func NewScanner(r io.Reader) *Scanner {
	reg, err := regexp.Compile(`\s*#include\s+["<]([^"<]*)([">])\s*`)
	if err != nil {
		panic(fmt.Sprintf("NewScanner Regexp.Compile threw and error: %q", err))
	}

	return &Scanner{
		scan:  bufio.NewScanner(r),
		regex: reg,
	}
}

func (s *Scanner) Scan() bool {
	s.text = ""
	for s.text == "" {
		if !s.scan.Scan() {
			return false
		}

		matches := s.regex.FindStringSubmatch(s.scan.Text())
		if len(matches) >= 3 && matches[1] != "" {
			s.text = matches[1]
			if matches[2] == ">" {
				s.typ = BracketIncludeType
			} else {
				s.typ = QuoteIncludeType
			}
		}
	}
	return true
}

func (s *Scanner) Text() string {
	return s.text
}

func (s *Scanner) Type() int {
	return s.typ
}
