package cppdep

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func init() {
	supressLogging = true
}

func TestSimpleCompile(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("main")

	c := &Compiler{OutputDir: outputDir}

	binaryPath, err := c.Compile(mainFile)

	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	_, err = os.Stat(filepath.Join(outputDir, "obj/main.o"))
	if err != nil {
		t.Errorf("main.o file was not created in output directory")
	}
	_, err = os.Stat(filepath.Join(outputDir, "obj/a.o"))
	if err != nil {
		t.Errorf("a.o file was not created in output directory")
	}
	_, err = os.Stat(filepath.Join(outputDir, "bin/main"))
	if err != nil {
		t.Errorf("main file was not created in output directory")
	}

	buf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Stdout = buf
	err = cmd.Run()
	switch {
	case err != nil:
		t.Errorf("Failed to execute %q", binaryPath)
	case string(buf.Bytes()) != "Hello World!\n":
		t.Errorf("Program output not as expect")
	}

	objCount := 0
	binCount := 0
	makeObjectHook = func(file *File) {
		objCount++
	}
	makeBinaryHook = func(file *File) {
		binCount++
	}
	defer func() {
		makeBinaryHook = nil
		makeObjectHook = nil
	}()

	_, err = c.Compile(mainFile)
	switch {
	case err != nil:
		t.Fatalf("Second compile failed: %v", err)
	case objCount != 0:
		t.Errorf("Expected no object files to be built: %d", objCount)
	case binCount != 0:
		t.Errorf("Expected no binary to be built: %d", binCount)
	}

	objCount = 0
	binCount = 0

	var ah *File
	for _, dep := range mainFile.Deps {
		if strings.LastIndex(dep.Path, "a.h") != -1 {
			ah = dep
			break
		}
	}

	// NOTE: unfortunately OS X timestamps only have 1 second resolution
	if runtime.GOOS == "darwin" {
		if testing.Short() {
			t.Skip("Skipping rest of the test because we need to sleep for a second on OS X")
		}
		time.Sleep(time.Second)
	}
	origModTime := ah.ModTime
	ah.ModTime = time.Now()
	if err := os.Chtimes(ah.Path, ah.ModTime, ah.ModTime); err != nil {
		t.Fatalf("Failed to modify times for %q", ah.Path)
	}

	_, err = c.Compile(mainFile)
	switch {
	case err != nil:
		t.Fatalf("Third compile failed: %v", err)
	case objCount != 2:
		t.Errorf("Expected two objects file to be built: %d", objCount)
	case binCount != 1:
		t.Errorf("Expected a binary to be built")
	}

	if runtime.GOOS == "darwin" {
		time.Sleep(time.Second)
	}
	ah.ModTime = origModTime
	os.Chtimes(ah.Path, ah.ModTime, ah.ModTime)
	objCount = 0
	binCount = 0

	mainFile.ModTime = time.Now()
	if err := os.Chtimes(mainFile.Path, mainFile.ModTime, mainFile.ModTime); err != nil {
		t.Fatalf("Failed to modify fimes for %q", mainFile.Path)
	}

	_, err = c.Compile(mainFile)
	switch {
	case err != nil:
		t.Fatalf("Fourth compile failed: %v", err)
	case objCount != 1:
		t.Errorf("Expected one object file to be built: %d", objCount)
	case binCount != 1:
		t.Errorf("Expected a binary to be built")
	}
}

func TestCompileSourceLib(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := &SourceTree{
		SrcRoot:    "test_files/source_lib",
		SourceLibs: map[string][]string{"lib.h": {"liba.cc", "libb.cc"}},
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("main")

	c := &Compiler{OutputDir: outputDir}
	_, err = c.Compile(mainFile)

	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
}

func TestSystemLibraryCompile(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := &SourceTree{
		SrcRoot:       "test_files/gzcat",
		LinkLibraries: map[string][]string{"zlib.h": {"-lz"}},
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("gzcat")

	c := &Compiler{OutputDir: outputDir}

	binaryPath, err := c.Compile(mainFile)

	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath, "test_files/gzcat/text.txt.gz")
	cmd.Stdout = buf
	err = cmd.Run()
	switch {
	case err != nil:
		t.Errorf("Failed to execute %q, %q", binaryPath, string(buf.Bytes()))
	case string(buf.Bytes()) != "This is a test file for gzcat.\n":
		t.Errorf("Program output not as expect")
	}
}

func TestCompileFlags(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := SourceTree{
		SrcRoot: "test_files/compiler_warning",
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("main")

	c := &Compiler{
		Flags:     []string{"-Wsign-compare", "-Werror"},
		OutputDir: outputDir,
	}

	_, err = c.Compile(mainFile)

	if err == nil {
		t.Errorf("Expected compile to fail due to warning and -Werror flag")
	}
}

func TestCompileUsingTypeGenerator(t *testing.T) {
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
		t.Fatalf("Unable to find main file")
	}

	c := &Compiler{
		IncludeDirs: []string{st.GenDir()},
		OutputDir:   outputDir,
	}
	_, err = c.Compile(mainFile)
	if err != nil {
		t.Errorf("Failed to compile: %v", err)
	}

	// TODO: test that one input being modified only triggers a single generator
}

func TestCompileUsingShellGenerator(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_generator_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	absShellPath, err := filepath.Abs("test_files/shell_generator/script.sh")
	if err != nil {
		t.Fatalf("Failed to get absolute path of shell script")
	}

	g := &ShellGenerator{
		InputPaths:    []string{"dir/firstHalf.txt", "dir/secondHalf.cc", "lib.h", "lib.cc"},
		OutputFiles:   []string{"main.cc", "modlib.h", "modlib.cc"},
		ShellFilePath: absShellPath,
	}

	st := &SourceTree{
		SrcRoot:    "test_files/shell_generator",
		Generators: []Generator{g},
		BuildDir:   outputDir,
	}
	st.ProcessDirectory()
	mainFile := st.FindSource("main")
	if mainFile == nil {
		t.Fatalf("Unable to find main file")
	}

	c := &Compiler{
		IncludeDirs: []string{st.GenDir()},
		OutputDir:   outputDir,
	}
	_, err = c.Compile(mainFile)
	if err != nil {
		t.Errorf("Failed to compile: %v", err)
	}
}

func TestCompileAllGeneratesObjectFilesOnce(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	files := []*File{st.FindSource("main"), st.FindSource("mainb")}

	c := &Compiler{OutputDir: outputDir}

	objCount := 0
	binCount := 0
	makeObjectHook = func(file *File) {
		objCount++
	}
	makeBinaryHook = func(file *File) {
		binCount++
	}
	defer func() {
		makeBinaryHook = nil
		makeObjectHook = nil
	}()

	_, err = c.CompileAll(files)
	switch {
	case err != nil:
		t.Errorf("CompileAll returned error: %v", err)
	case objCount != 3:
		t.Errorf("Expected 3 object files to be built, actually %d were built", objCount)
	case binCount != 2:
		t.Errorf("Expected 2 binary files to be built, actually %d were built", binCount)
	}
}

func TestCompileRenamedBinaries(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := SourceTree{
		SrcRoot: "test_files/simple",
	}
	st.ProcessDirectory()

	mainFile := st.FindSource("main")
	mainFile.BinaryName = "change_bin_name"

	c := &Compiler{OutputDir: outputDir}

	expectedBinPath := filepath.Join(outputDir, "bin", "change_bin_name")
	binaryPath, err := c.Compile(mainFile)
	switch {
	case err != nil:
		t.Errorf("Compile failed: %v", err)
	case binaryPath != expectedBinPath:
		t.Errorf("Output filename was not changed correctly: %q != %q", binaryPath, expectedBinPath)
	}

}
