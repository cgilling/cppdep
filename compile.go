package cppdep

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrCompilerError = errors.New("compiler returned an error")
)

type Compiler struct {
	IncludeDirs []string
	OutputDir   string

	Concurrency int
}

func (c *Compiler) Compile(file *File) (path string, err error) {
	var mu sync.Mutex
	var compileErr error
	var objList []string
	var libList []string

	concurrency := c.Concurrency
	if concurrency == 0 {
		concurrency = 1
	}

	deps := file.DepListFollowSource()
	deps = append(deps, file)

	var wg sync.WaitGroup
	depCh := make(chan *File)

	compileDepLoop := func() {
		for dep := range depCh {
			defer wg.Done()
			var path string
			var err error
			if dep.Type == SourceType {
				path, err = c.makeObject(dep)
			}

			if err != nil {
				mu.Lock()
				compileErr = err
				mu.Unlock()
				break
			}

			mu.Lock()
			if dep.Libs != nil {
				libList = append(libList, dep.Libs...)
			}
			if path != "" {
				objList = append(objList, path)
			}
			mu.Unlock()
		}
	}

	for i := 0; i < concurrency; i++ {
		go compileDepLoop()
	}

	for _, dep := range deps {
		wg.Add(1)
		depCh <- dep
	}
	close(depCh)
	wg.Wait()
	if compileErr != nil {
		return "", compileErr
	}

	return c.makeBinary(file, objList, libList)
}

func (c *Compiler) includeDirective() []string {
	var ids []string
	for _, dir := range c.IncludeDirs {
		ids = append(ids, "-I"+dir)
	}
	return ids
}

func (c *Compiler) makeObject(file *File) (path string, err error) {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	objectPath := filepath.Join(c.OutputDir, base[:dotIndex]+".o")
	cmd := exec.Command("g++", "-Wall", "-Wno-sign-compare", "-Wno-deprecated", "-Wno-write-strings", "-o", objectPath)
	cmd.Args = append(cmd.Args, c.includeDirective()...)
	cmd.Args = append(cmd.Args, "-c")
	cmd.Args = append(cmd.Args, file.Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Compiling: %s\n", filepath.Base(objectPath))
	err = cmd.Run()
	return objectPath, err
}

func (c *Compiler) makeBinary(file *File, objectPaths, libList []string) (path string, err error) {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	binaryPath := filepath.Join(c.OutputDir, base[:dotIndex])
	cmd := exec.Command("g++", "-o", binaryPath)
	cmd.Args = append(cmd.Args, libList...)
	cmd.Args = append(cmd.Args, objectPaths...)
	fmt.Printf("Compiling: %s\n", filepath.Base(binaryPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return binaryPath, err
}
