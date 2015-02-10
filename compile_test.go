package cppdep

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
}

func TestSystemLibraryCompile(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	st := &SourceTree{
		Libraries: map[string]string{"zlib.h": "-lz"},
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
