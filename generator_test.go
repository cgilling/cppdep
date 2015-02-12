package cppdep

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestTypeGenerator(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_generator_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	g := &TypeGenerator{
		InputExt:   ".txt",
		OutputExts: []string{".cc"},
		Command:    []string{"cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.cc"},
	}

	err = g.Generate("test_files/simple_generate/test.txt", outputDir)
	if err != nil {
		t.Fatalf("Failed to generate files: %v", err)
	}

	b, err := ioutil.ReadFile(fmt.Sprintf("%s/test.cc", outputDir))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	} else if string(b) != "Hello World!\n" {
		t.Errorf("Contents of file not as expected")
	}
}

func TestShellGenerator(t *testing.T) {
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

	testPaths := []string{"dir/firstHalf.txt", "dir/secondHalf.cc", "lib.h", "lib.cc", "myfile.cc", "myheader.h", "mylib.h"}
	expReturn := []bool{true, true, true, true, false, false, false}
	for i, path := range testPaths {
		ret := g.Match(path)
		if ret != expReturn[i] {
			t.Errorf("return for Match(%q) did not return as extected, returned %v", path, ret)
		}
	}

	outputPathHash := map[string]struct{}{
		filepath.Join(outputDir, g.OutputFiles[0]): {},
		filepath.Join(outputDir, g.OutputFiles[1]): {},
		filepath.Join(outputDir, g.OutputFiles[2]): {},
	}
	outputPaths := g.OutputPaths("", outputDir)
	for _, path := range outputPaths {
		if _, ok := outputPathHash[path]; !ok {
			t.Errorf("Unexpected file in OutputPaths: %q", path)
		}
	}
	if len(outputPaths) != 3 {
		t.Errorf("Too many files returned by OutputPaths, expected 4, got %d", len(outputPaths))
	}

	err = g.Generate("", outputDir)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}
	for _, path := range outputPaths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Failed to stat output file: %q", path)
		}
	}

}
