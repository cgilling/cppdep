package cppdep

import (
	"reflect"
	"strings"
	"testing"
)

func TestScanner(t *testing.T) {
	source :=
		`#include <mytest>
		 #include <stdio.h>
		 #define MY_CONSTANT 1
		 #include "localfile.h"
		 #include "subdir/file.h"`
	s := NewScanner(strings.NewReader(source))

	var includes []string
	var types []int
	for s.Scan() {
		includes = append(includes, s.Text())
		types = append(types, s.Type())
	}

	expectedIncludes := []string{
		"mytest",
		"stdio.h",
		"localfile.h",
		"subdir/file.h",
	}

	expectedTypes := []int{
		BracketIncludeType,
		BracketIncludeType,
		QuoteIncludeType,
		QuoteIncludeType,
	}

	if !reflect.DeepEqual(includes, expectedIncludes) {
		t.Errorf("Include list not as expected.\ngot:%v\nexp:%v\n", includes, expectedIncludes)
	}

	if !reflect.DeepEqual(types, expectedTypes) {
		t.Errorf("types list not as expected.\ngot:%v\nexp:%v\n", types, expectedTypes)
	}
}

func TestFastScanner(t *testing.T) {
	source :=
		`// This is my comment
		 #include <after_comment>

		 #include <after_blank_line.h>
		 #define MY_CONSTANT
		 #include "after_define.h"
		 #define MY_FUNC(a,b) \
		  (a \
		  + b)
		 #include "after_multi_line_define.h"
		 /* This is
		    a multi line
		    comment */
		 #include "after_multi_line_comment.h"

		 int myFunc(); // make sure this doesn't count as comment

		 #include <after_func.h>`
	s := NewFastScanner(strings.NewReader(source))

	var includes []string
	for s.Scan() {
		includes = append(includes, s.Text())
	}

	contains := func(list []string, target string) bool {
		for _, item := range list {
			if target == item {
				return true
			}
		}
		return false
	}

	switch {
	case contains(includes, "after_func.h"):
		t.Errorf("Found header after function define")
	case !contains(includes, "after_comment"):
		t.Error("Failed to find include following a single line comment")
	case !contains(includes, "after_blank_line.h"):
		t.Error("Failed to find include following an empty line")
	case !contains(includes, "after_define.h"):
		t.Error("Failed to find include following precompiler statement")
	case !contains(includes, "after_multi_line_define.h"):
		t.Error("Failed to find include following multi line precompiler statement")
	case !contains(includes, "after_multi_line_comment.h"):
		t.Error("Failed to find include following multi line comment")
	}

}
