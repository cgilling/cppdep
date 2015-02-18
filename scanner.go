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

var (
	includeRegex      *regexp.Regexp
	commentRegex      *regexp.Regexp
	whitespaceRegex   *regexp.Regexp
	precompRegex      *regexp.Regexp
	multiPrecompStart *regexp.Regexp
	multiPrecompCont  *regexp.Regexp
	multiCommentStart *regexp.Regexp
	multiCommentEnd   *regexp.Regexp
)

func init() {
	var err error
	if includeRegex, err = regexp.Compile(`\s*#include\s+["<]([^"<]*)([">])\s*`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if commentRegex, err = regexp.Compile(`^\s*//.*$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if whitespaceRegex, err = regexp.Compile(`^\s*$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if precompRegex, err = regexp.Compile(`^\s*#.*$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if multiPrecompStart, err = regexp.Compile(`^\s*#.*\\$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if multiPrecompCont, err = regexp.Compile(`^.*\\$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if multiCommentStart, err = regexp.Compile(`^\s*/\*.*$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
	if multiCommentEnd, err = regexp.Compile(`^.*\*/\s*$`); err != nil {
		panic(fmt.Sprintf("Regexp.Compile threw and error: %q", err))
	}
}

// Scanner is used to scan source files to look for include statements
type Scanner struct {
	scan *bufio.Scanner
	text string
	typ  int

	fastMode bool
}

func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		scan: bufio.NewScanner(r),
	}
}

// NewFastScanner returns a new Scanner set in fast mode. Fast mode assumes that
// includes will only occur at the top of the file in which other precompiler statements
// and comments are allowed but nothing else.
func NewFastScanner(r io.Reader) *Scanner {
	s := NewScanner(r)
	s.fastMode = true
	return s
}

func (s *Scanner) Scan() bool {
	if s.fastMode {
		return s.fastScan()
	} else {
		return s.fullScan()
	}
}

func (s *Scanner) fastScan() bool {
	var inMultiline bool
	var inMultiComment bool
	s.text = ""
	for s.text == "" {
		if !s.scan.Scan() {
			return false
		}

		if inMultiComment && !multiCommentEnd.MatchString(s.scan.Text()) {
			continue
		} else if inMultiComment {
			inMultiComment = false
			continue
		}

		matches := includeRegex.FindStringSubmatch(s.scan.Text())
		if len(matches) >= 3 && matches[1] != "" {
			s.text = matches[1]
			if matches[2] == ">" {
				s.typ = BracketIncludeType
			} else {
				s.typ = QuoteIncludeType
			}
		} else {
			switch {
			case multiPrecompStart.MatchString(s.scan.Text()):
				inMultiline = true
			case inMultiline && multiPrecompCont.MatchString(s.scan.Text()):
			case commentRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case whitespaceRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case precompRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case multiCommentStart.MatchString(s.scan.Text()):
				inMultiComment = true
			default:
				if inMultiline {
					inMultiline = false
				} else {
					return false
				}
			}
		}
	}
	return true
}

func (s *Scanner) fullScan() bool {
	s.text = ""
	for s.text == "" {
		if !s.scan.Scan() {
			return false
		}

		matches := includeRegex.FindStringSubmatch(s.scan.Text())
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
