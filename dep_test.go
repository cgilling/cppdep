package cppdep

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	mainFile := st.FindSource("main")
	aFile := st.FindSource("a")

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

	mainb := st.FindSource("mainb")
	if mainb != nil {
		t.Errorf("Failed to exclude directory")
	}
	main := st.FindSource("main")
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

	mainFile := st.FindSource("main")
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
	mainFile := st.FindSource("gzcat")
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
	mainFile := st.FindSource("main")
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
	mainFile := st.FindSource("main")
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

	sources, err := st.FindSources(`source_lib/lib*`)
	switch {
	case err != nil:
		t.Errorf("FindSources returned error: %v", err)
	case len(sources) != 2:
		t.Errorf("Expected two sources to be returned: got %d", len(sources))
	}

	sources, err = st.FindSources(`source_lib/lib`)
	switch {
	case err != nil:
		t.Errorf("Error on find sources #3: %v", err)
	case len(sources) != 0:
		t.Errorf("Expected not to fine any sources for partial match, found %d", len(sources))
	}
}

func TestFindMainFiles(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()
	mainFiles, err := st.FindMainFiles()
	switch {
	case err != nil:
		t.Errorf("FindMainFiles returned error: %v", err)
	case len(mainFiles) != 2:
		t.Errorf("Expected 2 main files to be found, got %d", len(mainFiles))
	case countEntries(mainFiles, "test_files/simple/main.cc") == 1:
		t.Errorf("Expected main.cc to be one of the main files")
	case countEntries(mainFiles, "test_files/simple/mainb.cc") == 1:
		t.Errorf("Expected mainb.cc to be one of the main files")
	}

	st = SourceTree{
		SrcRoot: "test_files/find_main_files",
	}
	st.ProcessDirectory()
	mainFiles, err = st.FindMainFiles()
	switch {
	case err != nil:
		t.Errorf("Failed to find main files: %v", err)
	case len(mainFiles) != 1:
		t.Errorf("Expected to only find 1 main file: got %d", len(mainFiles))
	}

	// TODO: need to test looking for main statements in files that seem like they
	// would be main files, as they could just be orphaned .cc files. (code not
	// implemented yet either)
}

func TestRename(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	rules := []RenameRule{
		{Regex: `(main)b`, Replace: `thebest$1`},
	}
	if err := st.Rename(rules); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	theBest := st.FindSource("thebestmain")
	main := st.FindSource("main")
	switch {
	case theBest == nil:
		t.Errorf("Unable to find renamed source")
	case main == nil:
		t.Errorf("Unable to find non renamed source")
	}

	files, err := st.FindSources("thebest*")
	switch {
	case err != nil:
		t.Errorf("FindSources returned error: %v", err)
	case len(files) != 1:
		t.Errorf("Exected FindSources to return 1 file: %d", len(files))
	}
}

func TestRenameNameCollision(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	rules := []RenameRule{
		{Regex: `(main)b`, Replace: `$1`},
	}
	st.Rename(rules)

	file := st.FindSource("main")
	switch {
	case file == nil:
		t.Errorf("could not file file for main")
	case !strings.HasSuffix(file.Path, "mainb.cc"):
		t.Errorf("Expected to find mainb.cc instead found: %q", filepath.Base(file.Path))
	}

	files, err := st.FindSources("mai*")
	switch {
	case err != nil:
		t.Errorf("FindSources returned error: %v", err)
	case len(files) != 1:
		t.Errorf("Expected to only find 1 source, found %d", len(files))
	}
}

func TestAutoInclude(t *testing.T) {
	st := SourceTree{
		SrcRoot:     "test_files/auto_include",
		ExcludeDirs: []string{"dirb"},
		AutoInclude: true,
	}
	st.ProcessDirectory()
	var foundA, foundB bool
	dira, dirb := filepath.Join(st.SrcRoot, "dira"), filepath.Join(st.SrcRoot, "dirb")
	for _, inc := range st.IncludeDirs {
		if inc == dira {
			foundA = true
		} else if inc == dirb {
			foundB = true
		}
	}
	if !foundA {
		t.Errorf("dira was not found in the include path")
	}
	if foundB {
		t.Errorf("dirb was found in the include path")
	}
}

func TestMultiRename(t *testing.T) {
	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	rules := []RenameRule{
		{Regex: `ma(in)`, Replace: `complete_change`},
		{Regex: `(main)b`, Replace: `thebest$1`},
	}
	if err := st.Rename(rules); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	main := st.FindSource("complete_change")
	mainb := st.FindSource("thebestmain")
	switch {
	case main == nil:
		t.Errorf("Failed to find file from first rule")
	case mainb == nil:
		t.Errorf("Failed to find file from second rule")
	}
}

func countEntries(list []*File, target string) int {
	count := 0
	for _, val := range list {
		if val.Path == target {
			count++
		}
	}
	return count
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

	depList := root.DepList()
	if countEntries(depList, "a.h") != 1 {
		t.Errorf("Expected DepList to contain a.h once, found %d times", countEntries(depList, "a.h"))
	}
	if countEntries(depList, "b.h") != 1 {
		t.Errorf("Expected DepList to contain b.h once, found %d times", countEntries(depList, "b.h"))
	}
}
