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
