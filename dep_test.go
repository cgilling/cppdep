package cppdep

import "testing"

func TestDepSimple(t *testing.T) {
	var st SourceTree
	st.ProcessDirectory("test_files/simple")

	mainFile := st.FindSource("main.cc")
	aFile := st.FindSource("a.cc")

	switch {
	case mainFile == nil:
		t.Fatalf("main.cc not among source list")
	case mainFile.Type != SourceType:
		t.Errorf("Expected main.cc to be source type")
	case aFile == nil:
		t.Fatalf("a.cc not amoung source list")
	case len(mainFile.Deps) != 1:
		t.Errorf("Expected to find one dependency for main.cc, found %d", len(mainFile.Deps))
	case mainFile.Deps[0].Path != "test_files/simple/a.h":
		t.Errorf("Did not find a.h as dependency for main.cc")
	case mainFile.Deps[0].Type != HeaderType:
		t.Errorf("Expected a.h to be header type")
	case mainFile.Deps[0].SourcePair != aFile:
		t.Errorf("source pair not found for a.h")
	}
}

func TestFileDepList(t *testing.T) {
	a := &File{Path: "a.h"}
	b := &File{Path: "b.h", Deps: []*File{a}}
	a.Deps = []*File{b}
	root := &File{
		Path: "main.cc",
		Deps: []*File{
			a,
			b,
		},
	}

	countEntries := func(list []*File, target string) int {
		count := 0
		for _, val := range list {
			if val.Path == target {
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
