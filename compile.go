package cppdep

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrCompilerError = errors.New("compiler returned an error")
)

type Compiler struct {
	IncludeDirs []string // include directories to be passed to compile
	Flags       []string // compile flags passed to the compiler

	// OutputDir is base output dir, object files written to OutputDir/obj
	// and compiled binaries will be written to OutputDir/bin
	OutputDir string

	Concurrency int // the number of concurrent compiles

	// Verbose when set to true will print out the compile statements being run
	Verbose bool
}

// BinPath returns the path where the binary for a given main file will be written.
func (c *Compiler) BinPath(file *File) string {
	var path string
	if file.BinaryName != "" {
		path = filepath.Join(c.OutputDir, "bin", file.BinaryName)
	} else {
		base := filepath.Base(file.Path)
		dotIndex := strings.LastIndex(base, ".")
		path = filepath.Join(c.OutputDir, "bin", base[:dotIndex])
	}
	if file.Type == LibType {
		path = path + ".so"
	}
	return path
}

// CompileAll will compile binaries whose main functions are defined by the entries
// in files. If there is any compile error for any of the binaries CompileAll will
// return false. Upon success the path of all the output binaries are returned in
// the same order as the input files.
func (c *Compiler) CompileAll(files []*File) (paths []string, err error) {
	if err := os.MkdirAll(filepath.Join(c.OutputDir, "bin"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(c.OutputDir, "obj"), 0755); err != nil {
		return nil, err
	}

	var sortedFiles []*File
	sortedFiles = append(sortedFiles, files...)
	sort.Sort(ByBase(sortedFiles))
	files = sortedFiles
	uniqueSources := make(map[string]*File)
	var fileSources [][]*File
	var fileLibs [][]string
	for _, file := range files {
		deps := file.DepListFollowSource()
		deps = append(deps, file)
		sources, libs := filterDeps(deps)
		for _, source := range sources {
			uniqueSources[source.Path] = source
		}
		fileSources = append(fileSources, sources)
		fileLibs = append(fileLibs, libs)
	}

	concurrency := c.Concurrency
	if concurrency == 0 {
		concurrency = 1
	}

	var mu sync.Mutex
	var compileErr error
	var wg sync.WaitGroup
	sourceCh := make(chan *File)

	compileDepLoop := func() {
		defer wg.Done()
		for source := range sourceCh {
			var err error

			mu.Lock()
			cerr := compileErr
			mu.Unlock()
			if cerr != nil {
				continue
			}
			_, err = c.makeObject(source)

			if err != nil {
				mu.Lock()
				compileErr = err
				mu.Unlock()
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go compileDepLoop()
	}

	var sortedSources []*File
	for _, source := range uniqueSources {
		sortedSources = append(sortedSources, source)
	}
	sort.Sort(ByBase(sortedSources))
	for _, source := range sortedSources {
		sourceCh <- source
	}
	close(sourceCh)
	wg.Wait()
	if compileErr != nil {
		return nil, compileErr
	}

	binCh := make(chan binaryInfo)
	var binWg sync.WaitGroup

	binWg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer binWg.Done()
			for binInfo := range binCh {
				mu.Lock()
				cerr := compileErr
				mu.Unlock()
				if cerr != nil {
					continue
				}

				var objects []string
				for _, file := range binInfo.sources {
					objects = append(objects, c.objectPath(file))
				}

				_, err := c.makeBinary(binInfo.file, objects, binInfo.libs)
				if err != nil {
					mu.Lock()
					compileErr = err
					mu.Unlock()
				}
			}
		}()
	}

	for i, file := range files {
		binCh <- binaryInfo{
			file:    file,
			sources: fileSources[i],
			libs:    fileLibs[i],
		}
	}
	close(binCh)
	binWg.Wait()
	if compileErr != nil {
		return nil, compileErr
	}

	var binPaths []string
	for _, file := range files {
		binPaths = append(binPaths, c.BinPath(file))
	}
	return binPaths, nil
}

func (c *Compiler) Compile(file *File) (path string, err error) {
	paths, err := c.CompileAll([]*File{file})
	if err != nil {
		return "", err
	}
	return paths[0], nil
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

func (c *Compiler) objectPath(file *File) string {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	return filepath.Join(c.OutputDir, "obj", base[:dotIndex]+".o")
}

func (c *Compiler) makeObject(file *File) (path string, err error) {
	objectPath := c.objectPath(file)

	var depPaths []string
	for _, dep := range append(file.DepList(), file) {
		if dep.Type == HeaderType || dep == file {
			depPaths = append(depPaths, dep.Path)
		}
	}

	// NOTE: in this instance we could speed this up by using the dep files
	//			 ModTime field. This would be faster, but wouldn't use this
	//			 generic method. Can address if it becomes an issue.
	needsCompile, err := needsRebuild(depPaths, []string{objectPath})
	if err != nil {
		return "", err
	} else if !needsCompile {
		return objectPath, nil
	}

	cmd := exec.Command("g++", "-o", objectPath)
	cmd.Args = append(cmd.Args, c.Flags...)
	cmd.Args = append(cmd.Args, c.includeDirective()...)
	cmd.Args = append(cmd.Args, "-c")
	cmd.Args = append(cmd.Args, file.Path)
	if !supressLogging {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if c.Verbose {
			fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
		} else {
			fmt.Printf("Compiling: %s\n", filepath.Base(objectPath))
		}

	}
	if makeObjectHook != nil {
		makeObjectHook(file)
	}
	err = cmd.Run()
	return objectPath, err
}

type binaryInfo struct {
	file    *File
	sources []*File
	libs    []string
}

func (c *Compiler) makeBinary(file *File, objectPaths, libList []string) (path string, err error) {
	binaryPath := c.BinPath(file)
	needsCompile, err := needsRebuild(objectPaths, []string{binaryPath})
	if err != nil {
		return "", err
	} else if !needsCompile {
		return binaryPath, nil
	}

	cmd := exec.Command("g++", "-o", binaryPath)
	if file.Type == LibType {
		cmd.Args = append(cmd.Args, "-shared")
	}
	cmd.Args = append(cmd.Args, c.Flags...)
	cmd.Args = append(cmd.Args, objectPaths...)
	cmd.Args = append(cmd.Args, libList...)
	if !supressLogging {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if c.Verbose {
			fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
		} else {
			fmt.Printf("Compiling: %s\n", filepath.Base(binaryPath))
		}
	}
	if makeBinaryHook != nil {
		makeBinaryHook(file)
	}
	err = cmd.Run()
	return binaryPath, err
}

func filterDeps(deps []*File) (sources []*File, libs []string) {
	for _, dep := range deps {
		if dep.Type == SourceType {
			sources = append(sources, dep)
		}
		if dep.Libs != nil {
			libs = append(libs, dep.Libs...)
		}
	}
	return sources, libs
}

func needsRebuild(inputPaths, outputPaths []string) (bool, error) {
	var inputModTime time.Time
	for _, path := range inputPaths {
		info, err := os.Stat(path)
		if err != nil {
			return false, err
		} else if info.ModTime().After(inputModTime) {
			inputModTime = info.ModTime()
		}
	}

	outputModTime := time.Now()
	for _, path := range outputPaths {
		info, err := os.Stat(path)
		if err != nil {
			outputModTime = time.Time{}
			break
		} else if info.ModTime().Before(outputModTime) {
			outputModTime = info.ModTime()
		}
	}
	return inputModTime.After(outputModTime), nil
}

type ByBase []*File

func (a ByBase) Len() int           { return len(a) }
func (a ByBase) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByBase) Less(i, j int) bool { return filepath.Base(a[i].Path) < filepath.Base(a[j].Path) }
