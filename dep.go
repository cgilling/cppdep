package cppdep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	HeaderType = iota + 1
	SourceType
	GenDepType
)

type SourceTree struct {
	IncludeDirs []string
	HeaderExts  []string
	SourceExts  []string

	// Libraries is a map that defines library header includes to the linker statement needed.
	// For example if #include <zlib.h> were in a file, this would be the approriate value to
	// be in the dictionary {"zlib.h": []string{"-lz"]}}. The value is a slice of strings so that
	// mutliple statements can be provided, the main purpose being if a library search path needs
	// to be added, which would look something like this: {"libpq-fe.h": ["-L/usr/pgsql-9.2/lib", "-lpq"]}
	Libraries map[string][]string

	Generators []*Generator

	// BuildDir is the directory where build files will be places. This is used
	// for a place to put output from the Generators.
	BuildDir string

	// Concurrency defines the number of goroutines used to concurrently
	// process dependencies. Default is 1.
	Concurrency int

	// UseFastScanning will use the NewFastScanner function for scanning documents rather
	// than the standard one. See the documention for Scanner for more information.
	UseFastScanning bool

	mu      sync.Mutex
	sources []*File
}

func (st *SourceTree) GenDir() string {
	return filepath.Join(st.BuildDir, "gen")
}

func (st *SourceTree) ProcessDirectory(rootDir string) error {
	if st.HeaderExts == nil {
		st.HeaderExts = []string{".h", ".hpp", ".hh", ".hxx"}
	}
	if st.SourceExts == nil {
		st.SourceExts = []string{".cc", ".c"}
	}
	if st.Concurrency == 0 {
		st.Concurrency = 1
	}
	if len(st.Generators) > 0 && st.BuildDir == "" {
		return fmt.Errorf("Build dir must be set if Generators are used")
	}

	// First collect all the files

	var genFiles []*genFile
	seen := make(map[string]*File)
	allExtsMap := make(map[string]struct{})
	for _, ext := range st.HeaderExts {
		allExtsMap[ext] = struct{}{}
	}
	for _, ext := range st.SourceExts {
		allExtsMap[ext] = struct{}{}
	}
	for _, gen := range st.Generators {
		allExtsMap[gen.InputExt] = struct{}{}
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if _, ok := allExtsMap[ext]; !ok {
			return nil
		}

		if _, ok := seen[path]; ok {
			return nil
		}

		for _, gen := range st.Generators {
			if ext == gen.InputExt {
				gf := &genFile{
					path:    path,
					modTime: info.ModTime(),
					gen:     gen,
				}
				genFiles = append(genFiles, gf)
				return nil
			}
		}

		file := &File{
			Path:    path,
			Type:    HeaderType,
			ModTime: info.ModTime(),
			stMu:    &st.mu,
		}
		seen[path] = file
		for _, sourceExt := range st.SourceExts {
			if ext == sourceExt {
				file.Type = SourceType
				st.sources = append(st.sources, file)
				break
			}
		}
		return nil
	}

	filepath.Walk(rootDir, walkFunc)

	// We need to run the generator here and add the output files to seen so they
	// can be picked up in the dependency graph

	genDir := st.GenDir()
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return err
	}
	for _, genFile := range genFiles {
		base := filepath.Base(genFile.path)
		dotIndex := strings.LastIndex(base, ".")
		outputPrefex := filepath.Join(genDir, base[:dotIndex])
		outModTime := time.Now()
		for _, outExt := range genFile.gen.OutputExts {
			info, err := os.Stat(fmt.Sprintf("%s%s", outputPrefex, outExt))
			if err != nil {
				outModTime = time.Time{}
			} else if info.ModTime().Before(outModTime) {
				outModTime = info.ModTime()
			}
		}
		if outModTime.Before(genFile.modTime) {
			genFile.gen.Generate(genFile.path, genDir)
		}
		for _, outExt := range genFile.gen.OutputExts {
			path := fmt.Sprintf("%s%s", outputPrefex, outExt)
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			walkFunc(path, info, nil)
		}
	}

	// Now scan all the files looking for includes and creating a dependency graph

	searchPath := []string{"placeholder", genDir}
	searchPath = append(searchPath, st.IncludeDirs...)

	processFile := func(file *File) error {
		fp, err := os.Open(file.Path)
		if err != nil {
			return err
		}
		var scan *Scanner
		if st.UseFastScanning {
			scan = NewFastScanner(fp)
		} else {
			scan = NewScanner(fp)
		}

		if file.Type == HeaderType {
			dotIndex := strings.LastIndex(file.Path, ".")
			for _, sourceExt := range st.SourceExts {
				testPath := file.Path[0:dotIndex] + sourceExt
				if pair, ok := seen[testPath]; ok {
					file.SourcePair = pair
					break
				}
			}
		}

		searchPath[0] = filepath.Dir(file.Path)

		for scan.Scan() {
			if scan.Type() == BracketIncludeType {
				if libs, ok := st.Libraries[scan.Text()]; ok {
					file.Libs = append(file.Libs, libs...)
				}
			}

			for _, dir := range searchPath {
				testPath := filepath.Join(dir, scan.Text())
				if depFile, ok := seen[testPath]; ok {
					file.Deps = append(file.Deps, depFile)
					break
				}
			}
		}

		fp.Close()

		return nil
	}

	ch := make(chan *File)
	var wg sync.WaitGroup

	for i := 0; i < st.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range ch {
				processFile(file)
			}
		}()
	}

	for _, file := range seen {
		ch <- file
	}
	close(ch)
	wg.Wait()
	return nil
}

func (st *SourceTree) FindSource(name string) *File {
	for _, file := range st.sources {
		if filepath.Base(file.Path) == name {
			return file
		}
	}
	return nil
}

type File struct {
	Path       string
	Deps       []*File
	Type       int
	SourcePair *File
	Libs       []string
	ModTime    time.Time

	// stMu used to ensure that only one goroutine is traversing the dependency
	// tree at any one time.
	stMu    *sync.Mutex
	visited bool
}

type genFile struct {
	path    string
	modTime time.Time
	gen     *Generator
}

// DepList will return the list of paths for all dependencies of f.
func (f *File) DepList() []*File {
	if f.stMu != nil {
		f.stMu.Lock()
		defer f.stMu.Unlock()
	}
	dl := f.generateDepList(false)
	f.unvisitDeps(false)
	return dl
}

// DepList will return the list of paths for all dependencies of f. It will follow
// the SourcePair of header files as well. Then intended use being that one could
// find all the files needed to compile a main .cc file.
func (f *File) DepListFollowSource() []*File {
	if f.stMu != nil {
		f.stMu.Lock()
		defer f.stMu.Unlock()
	}
	dl := f.generateDepList(true)
	f.unvisitDeps(true)
	return dl
}

func (f *File) generateDepList(followSource bool) []*File {
	f.visited = true
	var dl []*File

	for _, dep := range f.Deps {
		if dep.visited {
			continue
		}
		dl = append(dl, dep)
		dl = append(dl, dep.generateDepList(followSource)...)
	}
	if followSource && f.SourcePair != nil && !f.SourcePair.visited {
		dl = append(dl, f.SourcePair)
		dl = append(dl, f.SourcePair.generateDepList(followSource)...)
	}
	return dl
}

func (f *File) unvisitDeps(followSource bool) {
	if !f.visited {
		return
	}
	f.visited = false
	for _, dep := range f.Deps {
		dep.unvisitDeps(followSource)
	}
	if followSource && f.SourcePair != nil {
		f.SourcePair.unvisitDeps(followSource)
	}
}
