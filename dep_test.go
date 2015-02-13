package cppdep

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var cwd string

func init() {
	c, err := os.Getwd()
	if err != nil {
		panic("Unable to find cwd")
	}
	cwd = c
}

func TestDepSimple(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

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
	case mainFile.Deps[0].Path != filepath.Join(cwd, "test_files/simple/a.h"):
		t.Errorf("Did not find a.h as dependency for main.cc")
	case mainFile.Deps[0].Type != HeaderType:
		t.Errorf("Expected a.h to be header type")
	case len(mainFile.Deps[0].ImplFiles) != 1 || mainFile.Deps[0].ImplFiles[0] != aFile:
		t.Errorf("implementation file not found for a.h")
	}
}

func TestExcludeDirs(t *testing.T) {
	st := &SourceTree{
		SrcRoot:     "test_files/exclude_dir",
		ExcludeDirs: []string{"subdir"},
	}
	st.ProcessDirectory()

	mainb := st.FindSource("mainb.cc")
	if mainb != nil {
		t.Errorf("Failed to exclude directory")
	}
	main := st.FindSource("main.cc")
	if main == nil {
		t.Errorf("Failed find file in non excluded directory")
	}
}

func TestSourceLib(t *testing.T) {
	st := &SourceTree{
		SrcRoot:    "test_files/source_lib",
		SourceLibs: map[string][]string{"lib.h": {"liba.cc", "libb.cc"}},
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("main.cc")
	hFile := mainFile.DepList()[0]
	switch {
	case len(hFile.ImplFiles) != 2:
		t.Errorf("Expected lib.h to have to implementation files: actually has %d", len(hFile.ImplFiles))
	}
}

func TestDepSystemLibrary(t *testing.T) {
	st := &SourceTree{
		SrcRoot:   "test_files/gzcat",
		Libraries: map[string][]string{"zlib.h": {"-lz"}},
	}
	st.ProcessDirectory()
	mainFile := st.FindSource("gzcat.cc")
	if !reflect.DeepEqual(mainFile.Libs, []string{"-lz"}) {
		t.Errorf("Expected gzcat Libs to be -lz, actually: %v", mainFile.Libs)
	}
}

func TestUsingFastScanningOption(t *testing.T) {
	st := &SourceTree{
		SrcRoot:         "test_files/fast_scan_fail",
		UseFastScanning: true,
	}
	st.ProcessDirectory()
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
		SrcRoot:    "test_files/generator_compile",
		Generators: []Generator{cg, hg},
		BuildDir:   outputDir,
	}
	st.ProcessDirectory()
	mainFile := st.FindSource("main.cc")
	if mainFile == nil {
		t.Errorf("Unable to find main file")
	}

	genCount := 0
	genHook = func(input string) {
		genCount++
	}
	st2 := &SourceTree{
		SrcRoot:    "test_files/generator_compile",
		Generators: []Generator{cg, hg},
		BuildDir:   outputDir,
	}
	st2.ProcessDirectory()

	if genCount != 0 {
		t.Errorf("Expected no files to be generated because nothing was modified")
	}

	// TODO: test that one input being modified only triggers a single generator

	// TODO: test that a file matched by a generator is still available as a file
	// 			 for other to include
}

func TestFindSources(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files",
	}
	st.ProcessDirectory()

	sources, err := st.FindSources(`source_lib/lib.*\.cc`)
	switch {
	case err != nil:
		t.Errorf("FindSources returned error: %v", err)
	case len(sources) != 2:
		t.Errorf("Expected two sources to be returned: got %d", len(sources))
	}

	_, err = st.FindSources(`\x1`)
	if err == nil {
		t.Errorf("Expected FindSources to fail when given a bad regex")
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
