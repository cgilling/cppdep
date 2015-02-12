package cppdep

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrCompilerError = errors.New("compiler returned an error")

	errNoCompileNeeded = errors.New("Compile skipped because of no modifications")
)

type Compiler struct {
	IncludeDirs []string
	Flags       []string // compile flags passed to the compiler
	OutputDir   string

	Concurrency int
}

func (c *Compiler) Compile(file *File) (path string, err error) {
	var mu sync.Mutex
	var compileErr error
	var objList []string
	var libList []string
	var objWasBuilt bool

	concurrency := c.Concurrency
	if concurrency == 0 {
		concurrency = 1
	}

	deps := file.DepListFollowSource()
	deps = append(deps, file)

	var wg sync.WaitGroup
	depCh := make(chan *File)

	compileDepLoop := func() {
		defer wg.Done()
		for dep := range depCh {
			var path string
			var err error

			mu.Lock()
			cerr := compileErr
			mu.Unlock()
			if cerr != nil {
				continue
			}

			if dep.Type == SourceType {
				path, err = c.makeObject(dep)
			} else {
				err = errNoCompileNeeded
			}

			if err != nil {
				if err != errNoCompileNeeded {
					mu.Lock()
					compileErr = err
					mu.Unlock()
					break
				}
			}

			mu.Lock()
			if err == nil {
				objWasBuilt = true
			}
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
		wg.Add(1)
		go compileDepLoop()
	}

	for _, dep := range deps {
		depCh <- dep
	}
	close(depCh)
	wg.Wait()
	if compileErr != nil {
		return "", compileErr
	}

	if !objWasBuilt {
		return c.binPath(file), nil
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

var supressLogging bool

// These are intended for testing only
var (
	makeObjectHook func(file *File)
	makeBinaryHook func(file *File)
)

func (c *Compiler) makeObject(file *File) (path string, err error) {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	objectPath := filepath.Join(c.OutputDir, base[:dotIndex]+".o")

	needsCompile := false
	var modTime time.Time
	info, err := os.Stat(objectPath)
	if err == nil {
		modTime = info.ModTime()
	} else {
		needsCompile = true
	}

	deps := append(file.DepList(), file)
	for _, dep := range deps {
		if dep.Type != HeaderType && dep != file {
			continue
		}
		if dep.ModTime.After(modTime) {
			needsCompile = true
			break
		}
	}

	if !needsCompile {
		return objectPath, errNoCompileNeeded
	}

	cmd := exec.Command("g++", "-o", objectPath)
	cmd.Args = append(cmd.Args, c.Flags...)
	cmd.Args = append(cmd.Args, c.includeDirective()...)
	cmd.Args = append(cmd.Args, "-c")
	cmd.Args = append(cmd.Args, file.Path)
	if !supressLogging {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Compiling: %s\n", filepath.Base(objectPath))
	}
	if makeObjectHook != nil {
		makeObjectHook(file)
	}
	err = cmd.Run()
	return objectPath, err
}

func (c *Compiler) binPath(file *File) string {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	return filepath.Join(c.OutputDir, base[:dotIndex])
}

func (c *Compiler) makeBinary(file *File, objectPaths, libList []string) (path string, err error) {
	binaryPath := c.binPath(file)
	cmd := exec.Command("g++", "-o", binaryPath)
	cmd.Args = append(cmd.Args, c.Flags...)
	cmd.Args = append(cmd.Args, objectPaths...)
	cmd.Args = append(cmd.Args, libList...)
	if !supressLogging {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Compiling: %s\n", filepath.Base(binaryPath))
	}
	if makeBinaryHook != nil {
		makeBinaryHook(file)
	}
	err = cmd.Run()
	return binaryPath, err
}
