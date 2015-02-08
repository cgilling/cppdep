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

	sources []*file
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

	seen := make(map[string]*file)
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

		file := &file{path: path, typ: HeaderType}
		seen[path] = file
		for _, sourceExt := range st.SourceExts {
			if ext == sourceExt {
				file.typ = SourceType
				st.sources = append(st.sources, file)
				break
			}
		}
		return nil
	}

	filepath.Walk(rootDir, walkFunc)

	searchPath := []string{"placeholder"}
	searchPath = append(searchPath, st.IncludeDirs...)

	processFile := func(file *file) error {
		fp, err := os.Open(file.path)
		if err != nil {
			return err
		}
		scan := NewScanner(fp)

		if file.typ == HeaderType {
			dotIndex := strings.LastIndex(file.path, ".")
			for _, sourceExt := range st.SourceExts {
				testPath := file.path[0:dotIndex] + sourceExt
				if pair, ok := seen[testPath]; ok {
					file.sourcePair = pair
					break
				}
			}
		}

		searchPath[0] = filepath.Dir(file.path)

		for scan.Scan() {
			if scan.Type() == BracketIncludeType {
				continue
			}

			for _, dir := range searchPath {
				testPath := filepath.Join(dir, scan.Text())
				if depFile, ok := seen[testPath]; ok {
					file.deps = append(file.deps, depFile)
					break
				}
			}
		}

		fp.Close()

		return nil
	}

	ch := make(chan *file)
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

func findFile(files []*file, path string) *file {
	for _, file := range files {
		if filepath.Base(file.path) == path {
			return file
		}
	}
	return nil
}

type file struct {
	path       string
	deps       []*file
	typ        int
	sourcePair *file

	visited bool
}

// DepList will return the list of paths for all dependencies of f. This
// call modifies shared state in the owning SourceTree, so it should not
// be called concurrently on multiple files.
func (f *file) DepList() []*file {
	dl := f.generateDepList()
	f.unvisitDeps()
	return dl
}

func (f *file) generateDepList() []*file {
	f.visited = true
	var dl []*file

	for _, dep := range f.deps {
		if dep.visited {
			continue
		}
		dl = append(dl, dep)
		dl = append(dl, dep.generateDepList()...)
	}
	return dl
}

func (f *file) unvisitDeps() {
	if !f.visited {
		return
	}
	f.visited = false
	for _, dep := range f.deps {
		dep.unvisitDeps()
	}
}
