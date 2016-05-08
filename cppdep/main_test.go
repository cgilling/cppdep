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
linklibraries:
  "zlib.h": ["-lz"]
modes:
  hello:
    flags: ["-DHELLO"]
typegenerators:
  -
    inputext: ".txtcc"
    outputexts: [".cc"]
    command: ["cp", "$CPPDEP_INPUT_FILE", "$CPPDEP_OUTPUT_PREFIX.cc"]
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
	defaultArgs := []string{
		"cppdep",
		"--fast",
		"--config",
		confPath,
	}
	makeCommandAndRun(defaultArgs)

	defaultPath := filepath.Join(outputDir, "default/bin/main")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Errorf("file not found where expected: %q", defaultPath)
	}
	binPath := filepath.Join(outputDir, "bin/main")
	if path, err := os.Readlink(binPath); err != nil {
		t.Errorf("Failed to read link for file in root binary directory: %v", err)
	} else if path != "../default/bin/main" {
		t.Errorf("root binary not linked to correct file: %q != %q", path, defaultPath)
	}

	helloArgs := make([]string, len(defaultArgs))
	copy(helloArgs, defaultArgs)
	helloArgs = append(helloArgs, "--mode", "hello")
	makeCommandAndRun(helloArgs)

	helloPath := filepath.Join(outputDir, "hello/bin/main")
	if _, err := os.Stat(helloPath); err != nil {
		t.Errorf("file not found where expected: %q", helloPath)
	}
	if path, err := os.Readlink(binPath); err != nil {
		t.Errorf("Failed to read link for file in root binary directory: %v", err)
	} else if path != "../hello/bin/main" {
		t.Errorf("root binary not linked to correct file: %q != %q", path, helloPath)
	}

	buf := &bytes.Buffer{}
	cmd := exec.Command(binPath)
	cmd.Stdout = buf
	cmd.Run()
	if string(buf.Bytes()) != "Hello World!\n" {
		t.Errorf("Flags not passed to compilation correctly.\ngot: %q\nexp: %q", buf.Bytes(), "Hello World!\n")
	}

	// This tests makes sure that even if nothing new was compiled, the symlink still points to the correct place
	makeCommandAndRun(defaultArgs)
	if path, err := os.Readlink(binPath); err != nil {
		t.Errorf("Failed to read link for file in root binary directory: %v", err)
	} else if path != "../default/bin/main" {
		t.Errorf("root binary not linked to correct file: %q != %q", path, defaultPath)
	}
}
