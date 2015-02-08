package cppdep

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	HeaderType = iota + 1
	SourceType
)

type SourceTree struct {
	IncludeDirs []string
	HeaderExts  []string
	SourceExts  []string

	// Concurrency defines the number of goroutines used to concurrently
	// process dependencies. Default is 1.
	Concurrency int

	sources []*File
}

func (st *SourceTree) ProcessDirectory(rootDir string) error {
	if st.HeaderExts == nil {
		st.HeaderExts = []string{".h"}
	}
	if st.SourceExts == nil {
		st.SourceExts = []string{".cc", ".c"}
	}
	if st.Concurrency == 0 {
		st.Concurrency = 1
	}

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
			return nil
		}

		ext := filepath.Ext(path)
		if _, ok := allExtsMap[ext]; !ok {
			return nil
		}

		if _, ok := seen[path]; ok {
			return nil
		}

		file := &File{Path: path, Type: HeaderType}
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

	searchPath := []string{"placeholder"}
	searchPath = append(searchPath, st.IncludeDirs...)

	processFile := func(file *File) error {
		fp, err := os.Open(file.Path)
		if err != nil {
			return err
		}
		scan := NewScanner(fp)

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
				continue
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

func findFile(files []*File, path string) *File {
	for _, file := range files {
		if filepath.Base(file.Path) == path {
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

	visited bool
}

// DepList will return the list of paths for all dependencies of f. This
// call modifies shared state in the owning SourceTree, so it should not
// be called concurrently on multiple files.
func (f *File) DepList() []*File {
	dl := f.generateDepList()
	f.unvisitDeps()
	return dl
}

func (f *File) generateDepList() []*File {
	f.visited = true
	var dl []*File

	for _, dep := range f.Deps {
		if dep.visited {
			continue
		}
		dl = append(dl, dep)
		dl = append(dl, dep.generateDepList()...)
	}
	return dl
}

func (f *File) unvisitDeps() {
	if !f.visited {
		return
	}
	f.visited = false
	for _, dep := range f.Deps {
		dep.unvisitDeps()
	}
}
