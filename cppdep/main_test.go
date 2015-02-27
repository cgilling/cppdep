package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
)

func TestMain(t *testing.T) {
	outputDir, err := ioutil.TempDir("", "cppdep_compile_test")
	if err != nil {
		t.Fatalf("Failed to setup output dir")
	}
	defer os.RemoveAll(outputDir)

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to Getwd(): %v", err)
	}
	srcDir := filepath.Join(pwd, "test_files")

	confPath := filepath.Join(outputDir, "cppdep.yml")
	confFile, err := os.Create(confPath)
	if err != nil {
		t.Fatalf("Failed to open config file for writing: %q (%v)", confPath, err)
	}
	confTmpl, err := template.New("config").Parse(`
srcdir: {{.SourceDir}}
builddir: {{.BuildDir}}
autoinclude: true
libraries:
  "zlib.h": ["-lz"]
modes:
  "hello":
    "flags": ["-DHELLO"]
typegenerators:
  -
    inputext: ".txtcc"
    outputexts: [".cc"]
    command: ["cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.cc"]
typegenerators:
  -
    inputext: ".txth"
    outputexts: [".h"]
    command: ["cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.h"]`)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}
	tmplParams := map[string]string{
		"BuildDir":  outputDir,
		"SourceDir": srcDir,
	}
	if err = confTmpl.Execute(confFile, tmplParams); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}
	args := []string{
		"cppdep",
		"--fast",
		"--config",
		confPath,
	}
	makeCommandAndRun(args)

	defaultPath := filepath.Join(outputDir, "default/bin/main")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Errorf("file not found where expected: %q", defaultPath)
	}

	args = append(args, "--mode", "hello")
	makeCommandAndRun(args)

	helloPath := filepath.Join(outputDir, "hello/bin/main")
	if _, err := os.Stat(helloPath); err != nil {
		t.Errorf("file not found where expected: %q", defaultPath)
	}
	buf := &bytes.Buffer{}
	cmd := exec.Command(helloPath)
	cmd.Stdout = buf
	cmd.Run()
	if string(buf.Bytes()) != "Hello World!\n" {
		t.Errorf("Flags not passed to compilation correctly.\ngot: %q\nexp: %q", buf.Bytes(), "Hello World!\n")
	}

}
