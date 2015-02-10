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

	fastMode          bool
	commentRegex      *regexp.Regexp
	whitespaceRegex   *regexp.Regexp
	precompRegex      *regexp.Regexp
	multiPrecompStart *regexp.Regexp
	multiPrecompCont  *regexp.Regexp
	multiCommentStart *regexp.Regexp
	multiCommentEnd   *regexp.Regexp
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

// NewFastScanner returns a new Scanner set in fast mode. Fast mode assumes that
// includes will only occur at the top of the file in which other precompiler statements
// and comments are allowed but nothing else.
func NewFastScanner(r io.Reader) *Scanner {
	var err error
	s := NewScanner(r)
	s.fastMode = true
	if s.commentRegex, err = regexp.Compile(`^\s*//.*$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.whitespaceRegex, err = regexp.Compile(`^\s*$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.precompRegex, err = regexp.Compile(`^\s*#.*$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.multiPrecompStart, err = regexp.Compile(`^\s*#.*\\$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.multiPrecompCont, err = regexp.Compile(`^.*\\$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.multiCommentStart, err = regexp.Compile(`^\s*/\*.*$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
	if s.multiCommentEnd, err = regexp.Compile(`^.*\*/\s*$`); err != nil {
		panic(fmt.Sprintf("NewFastScanner Regexp.Compile threw and error: %q", err))
	}
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

		if inMultiComment && !s.multiCommentEnd.MatchString(s.scan.Text()) {
			continue
		} else if inMultiComment {
			inMultiComment = false
			continue
		}

		matches := s.regex.FindStringSubmatch(s.scan.Text())
		if len(matches) >= 3 && matches[1] != "" {
			s.text = matches[1]
			if matches[2] == ">" {
				s.typ = BracketIncludeType
			} else {
				s.typ = QuoteIncludeType
			}
		} else {
			switch {
			case s.multiPrecompStart.MatchString(s.scan.Text()):
				inMultiline = true
			case inMultiline && s.multiPrecompCont.MatchString(s.scan.Text()):
			case s.commentRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case s.whitespaceRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case s.precompRegex.MatchString(s.scan.Text()):
				inMultiline = false
			case s.multiCommentStart.MatchString(s.scan.Text()):
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
