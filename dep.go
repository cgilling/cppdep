package cppdep

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	HeaderType = iota + 1
	SourceType
	GenDepType
	LibType
)

type SourceTree struct {
	SrcRoot     string
	IncludeDirs []string
	ExcludeDirs []string
	HeaderExts  []string
	SourceExts  []string

	// Libraries is a map of library name to a list of source files that will be
	// compiled together (along with all necessary dependencies) to create a shared library
	Libraries map[string][]string

	// LinkLibraries is a map that defines library header includes to the linker statement needed.
	// For example if #include <zlib.h> were in a file, this would be the approriate value to
	// be in the dictionary {"zlib.h": []string{"-lz"]}}. The value is a slice of strings so that
	// mutliple statements can be provided, the main purpose being if a library search path needs
	// to be added, which would look something like this: {"libpq-fe.h": ["-L/usr/pgsql-9.2/lib", "-lpq"]}
	LinkLibraries map[string][]string

	// SourceLibs are a way of defining a relationship where a single.h file is implemented by multiple
	// source files in the source tree. The key is the path to the header file relative to the root of the
	// source tree and the key is the list of source file paths relative to the root of the source tree.
	SourceLibs map[string][]string

	Generators []Generator

	// BuildDir is the directory where build files will be places. This is used
	// for a place to put output from the Generators.
	BuildDir string

	// Concurrency defines the number of goroutines used to concurrently
	// process dependencies. Default is 1.
	Concurrency int

	// UseFastScanning will use the NewFastScanner function for scanning documents rather
	// than the standard one. See the documention for Scanner for more information.
	UseFastScanning bool

	// AutoInclude will automatically add all directories found while scanning the source
	// tree to IncludeDirs. Directories found in ExcludeDirs and all sub directories will
	// be excluded.
	AutoInclude bool

	mu      sync.Mutex
	sources []*File
}

func (st *SourceTree) GenDir() string {
	return filepath.Join(st.BuildDir, "gen")
}

type RenameRule struct {
	Regex   string
	Replace string
}

// Rename take the rules passed in and applies them to all sources found
// during the ProcessDirectory call. The Regex must match the entire binary
// name to be applied. If multiple rules apply to a single binary, then the
// first one to match will apply. The default binary name is found by
// calling filepath.Base(file.Path) and then removing the extension.
func (st *SourceTree) Rename(rules []RenameRule) error {
	var regexps []*regexp.Regexp
	for _, rn := range rules {
		reg, err := regexp.Compile(rn.Regex)
		if err != nil {
			return err
		}
		regexps = append(regexps, reg)
	}
	for _, file := range st.sources {
		name := removeExt(filepath.Base(file.Path))
		for i, reg := range regexps {
			loc := reg.FindStringIndex(name)
			if loc != nil && loc[0] == 0 && loc[1] == len(name) {
				file.BinaryName = reg.ReplaceAllString(name, rules[i].Replace)
				break
			}
		}
	}
	return nil
}

// setup ensures needed paths are absolute, and sets up default values
func (st *SourceTree) setup() error {
	if st.SrcRoot == "" {
		return fmt.Errorf("SrcDir must not be empty")
	}
	absSrcRoot, err := filepath.Abs(st.SrcRoot)
	if err != nil {
		return err
	}
	st.SrcRoot = absSrcRoot

	for i, inc := range st.IncludeDirs {
		if !filepath.IsAbs(inc) {
			st.IncludeDirs[i] = filepath.Join(st.SrcRoot, inc)
		}
	}
	for i, ex := range st.ExcludeDirs {
		if !filepath.IsAbs(ex) {
			st.ExcludeDirs[i] = filepath.Join(st.SrcRoot, ex)
		}
	}

	if st.HeaderExts == nil {
		st.HeaderExts = []string{".h", ".hpp", ".hh", ".hxx"}
	}
	if st.SourceExts == nil {
		st.SourceExts = []string{".cc", ".cxx", ".c"}
	}
	if st.Concurrency == 0 {
		st.Concurrency = 1
	}
	if len(st.Generators) > 0 && st.BuildDir == "" {
		return fmt.Errorf("Build dir must be set if Generators are used")
	}
	return nil
}

func (st *SourceTree) ProcessDirectory() error {
	if err := st.setup(); err != nil {
		return err
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

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, dir := range st.ExcludeDirs {
				if path == dir {
					return filepath.SkipDir
				}
			}
			if st.AutoInclude {
				st.IncludeDirs = append(st.IncludeDirs, path)
			}
			return nil
		}

		for _, gen := range st.Generators {
			if gen.Match(path) {
				gf := &genFile{
					path:    path,
					modTime: info.ModTime(),
					gen:     gen,
				}
				genFiles = append(genFiles, gf)
			}
		}

		ext := filepath.Ext(path)
		if _, ok := allExtsMap[ext]; !ok {
			return nil
		}

		if _, ok := seen[path]; ok {
			return nil
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

	filepath.Walk(st.SrcRoot, walkFunc)

	// We need to run the generator here and add the output files to seen so they
	// can be picked up in the dependency graph

	// NOTE: for generators that take more than one file for input, this will do
	// 		 a bunch of extra os.Stat calls but shouldn't generate multiple times.

	genDir := st.GenDir()
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return err
	}
	st.IncludeDirs = append(st.IncludeDirs, genDir)
	for _, genFile := range genFiles {
		outModTime := time.Now()
		outputPaths := genFile.gen.OutputPaths(genFile.path, genDir)
		for _, outPath := range outputPaths {
			info, err := os.Stat(outPath)
			if err != nil {
				outModTime = time.Time{}
			} else if info.ModTime().Before(outModTime) {
				outModTime = info.ModTime()
			}
		}
		if outModTime.Before(genFile.modTime) {
			genFile.gen.Generate(genFile.path, genDir)
		}
		for _, outPath := range outputPaths {
			info, err := os.Stat(outPath)
			if err != nil {
				return err
			}
			walkFunc(outPath, info, nil)
		}
	}

	for libname, sources := range st.Libraries {
		var depList []*File
		for _, source := range sources {
			path := filepath.Join(st.SrcRoot, source)
			dep, ok := seen[path]
			if !ok {
				return fmt.Errorf("Unable to find source (%s) for library %q", source, libname)
			}
			depList = append(depList, dep)
		}
		file := &File{
			Type:       LibType,
			BinaryName: libname,
			Deps:       depList,
		}
		st.sources = append(st.sources, file)
	}

	// Now scan all the files looking for includes and creating a dependency graph

	sourceLibs := make(map[string][]string)
	for hpath, implPaths := range st.SourceLibs {
		var paths []string
		for _, p := range implPaths {
			paths = append(paths, filepath.Join(st.SrcRoot, p))
		}
		sourceLibs[filepath.Join(st.SrcRoot, hpath)] = paths
	}

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
			for hpath, implPaths := range sourceLibs {
				if hpath == file.Path {
					for _, path := range implPaths {
						if implFile, ok := seen[path]; ok {
							file.ImplFiles = append(file.ImplFiles, implFile)
						}
					}
					break
				}
			}
			if len(file.ImplFiles) == 0 {
				dotIndex := strings.LastIndex(file.Path, ".")
				for _, sourceExt := range st.SourceExts {
					testPath := file.Path[0:dotIndex] + sourceExt
					if pair, ok := seen[testPath]; ok {
						file.ImplFiles = append(file.ImplFiles, pair)
						break
					}
				}
			}
		}

		searchPath := []string{filepath.Dir(file.Path)}
		searchPath = append(searchPath, st.IncludeDirs...)

		for scan.Scan() {
			if scan.Type() == BracketIncludeType {
				if libs, ok := st.LinkLibraries[scan.Text()]; ok {
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

func removeExt(path string) string {
	extPos := strings.LastIndex(path, ".")
	if extPos == -1 {
		return path
	}
	return path[:extPos]
}

func (st *SourceTree) FindSource(name string) *File {
	for _, file := range st.sources {
		if file.BinaryName == name {
			return file
		}
	}
	for _, file := range st.sources {
		if filepath.Base(removeExt(file.Path)) == name {
			return file
		}
	}
	return nil
}

// FindSources uses a file globbing patters (as defined by filepath.Match) and finds
// all sources files that match that pattern. If pattern does not contain a file
// separator character, then jus the binary name is matched against. If it contains
// a file separator, then it is assumed to be relative to the root of the source
// tree.
func (st *SourceTree) FindSources(pattern string) ([]*File, error) {
	foundNames := make(map[string]struct{})
	var sources []*File

	useFullPath := false
	if strings.Index(pattern, string(filepath.Separator)) != -1 {
		useFullPath = true
		pattern = filepath.Join(st.SrcRoot, pattern)
	}

	if _, err := filepath.Match(pattern, "testthis"); err != nil {
		return nil, err
	}

	add_file := func(file *File) {
		var bn string
		if file.BinaryName != "" {
			bn = filepath.Join(filepath.Dir(file.Path), file.BinaryName)
		} else {
			bn = removeExt(file.Path)
		}
		base := filepath.Base(bn)
		if _, found := foundNames[base]; found {
			return
		}
		if !useFullPath {
			bn = base
		}
		if matched, _ := filepath.Match(pattern, bn); matched {
			foundNames[base] = struct{}{}
			sources = append(sources, file)
		}
	}

	for _, file := range st.sources {
		if file.BinaryName == "" {
			continue
		}
		add_file(file)
	}
	for _, file := range st.sources {
		if file.BinaryName != "" {
			continue
		}
		add_file(file)
	}
	return sources, nil
}

type inVectorValue struct {
	file  *File
	count int
}

func (st *SourceTree) FindMainFiles() ([]*File, error) {
	inVectors := make(map[string]inVectorValue)
	for _, file := range st.sources {
		if _, ok := inVectors[file.Path]; !ok {
			inVectors[file.Path] = inVectorValue{file: file}
		}
		deps := file.DepListFollowSource()
		for _, dep := range deps {
			if _, ok := inVectors[dep.Path]; !ok {
				inVectors[dep.Path] = inVectorValue{file: dep}
			}
			v := inVectors[dep.Path]
			v.count++
			inVectors[dep.Path] = v
		}
	}

	mainRegexp, err := regexp.Compile(`\s*int\s*main\s*\(\s*int\s+[a-zA-Z]+\s*,\s*char\s*\*\*?\s*[a-zA-Z]+\s*\[?\s*\]?\s*\)\s*{`)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	var files []*File
	var readErr error
	var wg sync.WaitGroup
	fileCh := make(chan *File)
	for i := 0; i < st.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileCh {
				fp, _ := os.Open(file.Path)
				if err != nil {
					mu.Lock()
					readErr = err
					mu.Unlock()
					continue
				}
				if mainRegexp.MatchReader(bufio.NewReader(fp)) {
					mu.Lock()
					files = append(files, file)
					mu.Unlock()
				}
				fp.Close()
			}
		}()
	}
	for _, v := range inVectors {
		if v.count == 0 {
			fileCh <- v.file
		}
	}
	close(fileCh)
	wg.Wait()
	return files, readErr
}

type File struct {
	Path       string
	Deps       []*File
	Type       int
	ImplFiles  []*File // the list of files that implement the functionality defined in this file
	Libs       []string
	ModTime    time.Time
	BinaryName string

	// stMu used to ensure that only one goroutine is traversing the dependency
	// tree at any one time.
	stMu    *sync.Mutex
	visited bool
}

type genFile struct {
	path    string
	modTime time.Time
	gen     Generator
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
// the ImplFiles of header files as well. Then intended use being that one could
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
	if followSource && f.ImplFiles != nil {
		for _, source := range f.ImplFiles {
			if !source.visited {
				dl = append(dl, source)
				dl = append(dl, source.generateDepList(followSource)...)
			}
		}
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
	if followSource && f.ImplFiles != nil {
		for _, source := range f.ImplFiles {
			source.unvisitDeps(followSource)
		}
	}
}
