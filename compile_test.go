package cppdep

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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

	var st SourceTree
	st.ProcessDirectory("test_files/simple")

	mainFile := st.FindSource("main.cc")

	c := &Compiler{OutputDir: outputDir}

	binaryPath, err := c.Compile(mainFile)

	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	_, err = os.Stat(filepath.Join(outputDir, "main.o"))
	if err != nil {
		t.Errorf("main.o file was not created in output directory")
	}
	_, err = os.Stat(filepath.Join(outputDir, "a.o"))
	if err != nil {
		t.Errorf("a.o file was not created in output directory")
	}
	_, err = os.Stat(filepath.Join(outputDir, "main"))
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

	_, err = c.Compile(mainFile)
	switch {
	case err != nil:
		t.Fatalf("Second compile failed: %v", err)
	case objCount != 0:
		t.Errorf("Expected no object files to be built")
	case binCount != 0:
		t.Errorf("Expected no binary to be built")
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
	origModTime := ah.ModTime
	ah.ModTime = time.Now()

	_, err = c.Compile(mainFile)
	switch {
	case err != nil:
		t.Fatalf("Third compile failed: %v", err)
	case objCount != 2:
		t.Errorf("Expected two objects file to be built: %d", objCount)
	case binCount != 1:
		t.Errorf("Expected a binary to be built")
	}

	ah.ModTime = origModTime
	objCount = 0
	binCount = 0

	mainFile.ModTime = time.Now()

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

func TestSystemLibraryCompile(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := &SourceTree{
		Libraries: map[string][]string{"zlib.h": []string{"-lz"}},
	}
	st.ProcessDirectory("test_files/gzcat")

	mainFile := st.FindSource("gzcat.cc")

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

	var st SourceTree
	st.ProcessDirectory("test_files/compiler_warning")

	mainFile := st.FindSource("main.cc")

	c := &Compiler{
		Flags:     []string{"-Wsign-compare", "-Werror"},
		OutputDir: outputDir,
	}

	_, err = c.Compile(mainFile)

	if err == nil {
		t.Errorf("Expected compile to fail due to warning and -Werror flag")
	}
}
