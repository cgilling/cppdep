package cppdep

import "testing"

func TestDepSimple(t *testing.T) {
	var st SourceTree
	st.ProcessDirectory("test_files/simple")

	mainFile := findFile(st.sources, "main.cc")
	aFile := findFile(st.sources, "a.cc")

	switch {
	case mainFile == nil:
		t.Fatalf("main.cc not among source list")
	case mainFile.typ != SourceType:
		t.Errorf("Expected main.cc to be source type")
	case aFile == nil:
		t.Fatalf("a.cc not amoung source list")
	case len(mainFile.deps) != 1:
		t.Errorf("Expected to find one dependency for main.cc, found %d", len(mainFile.deps))
	case mainFile.deps[0].path != "test_files/simple/a.h":
		t.Errorf("Did not find a.h as dependency for main.cc")
	case mainFile.deps[0].typ != HeaderType:
		t.Errorf("Expected a.h to be header type")
	case mainFile.deps[0].sourcePair != aFile:
		t.Errorf("source pair not found for a.h")
	}
}

func TestFileDepList(t *testing.T) {
	a := &file{path: "a.h"}
	b := &file{path: "b.h", deps: []*file{a}}
	a.deps = []*file{b}
	root := &file{
		path: "main.cc",
		deps: []*file{
			a,
			b,
		},
	}

	countEntries := func(list []*file, target string) int {
		count := 0
		for _, val := range list {
			if val.path == target {
				count++
			}
		}
		return count
	}

	depList := root.DepList()
	if countEntries(depList, "a.h") != 1 {
		t.Errorf("Expected DepList to contain a.h once, found %d times", countEntries(depList, "a.h"))
	}
	if countEntries(depList, "b.h") != 1 {
		t.Errorf("Expected DepList to contain b.h once, found %d times", countEntries(depList, "b.h"))
	}
}
