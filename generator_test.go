package cppdep

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestSimpleGenerator(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_generator_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	g := &Generator{
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
