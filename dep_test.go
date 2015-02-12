package cppdep

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

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

func TestDepSystemLibrary(t *testing.T) {
	st := &SourceTree{
		Libraries: map[string][]string{"zlib.h": {"-lz"}},
	}
	st.ProcessDirectory("test_files/gzcat")
	mainFile := st.FindSource("gzcat.cc")
	if !reflect.DeepEqual(mainFile.Libs, []string{"-lz"}) {
		t.Errorf("Expected gzcat Libs to be -lz, actually: %v", mainFile.Libs)
	}
}

func TestUsingFastScanningOption(t *testing.T) {
	st := &SourceTree{UseFastScanning: true}
	st.ProcessDirectory("test_files/fast_scan_fail")
	mainFile := st.FindSource("main.cc")
	if len(mainFile.Deps) != 0 {
		t.Errorf("Picked up an deps that weren't expected: %v", mainFile.Deps)
	}
}

func TestUsingTypeGenerator(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	cg := &TypeGenerator{
		InputExt:   ".txtc",
		OutputExts: []string{".cc"},
		Command:    []string{"cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.cc"},
	}
	hg := &TypeGenerator{
		InputExt:   ".txth",
		OutputExts: []string{".h"},
		Command:    []string{"cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.h"},
	}
	st := &SourceTree{
		Generators: []Generator{cg, hg},
		BuildDir:   outputDir,
	}
	st.ProcessDirectory("test_files/generator_compile")
	mainFile := st.FindSource("main.cc")
	if mainFile == nil {
		t.Errorf("Unable to find main file")
	}

	genCount := 0
	genHook = func(input string) {
		genCount++
	}
	st2 := &SourceTree{
		Generators: []Generator{cg, hg},
		BuildDir:   outputDir,
	}
	st2.ProcessDirectory("test_files/generator_compile")

	if genCount != 0 {
		t.Errorf("Expected no files to be generated because nothing was modified")
	}

	// TODO: test that one input being modified only triggers a single generator

	// TODO: test that a file matched by a generator is still available as a file
	// 			 for other to include
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
