package cppdep

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrCompilerError = errors.New("compiler returned an error")
)

type Compiler struct {
	OutputDir string
}

func (c *Compiler) Compile(file *File) (path string, err error) {
	deps := file.DepList()
	deps = append(deps, file)

	var objList []string
	for _, dep := range deps {
		var path string
		var err error
		if dep.Type == SourceType {
			path, err = c.makeObject(dep)
		} else if dep.Type == HeaderType && dep.SourcePair != nil {
			path, err = c.makeObject(dep.SourcePair)
		}
		if err != nil {
			return "", err
		}
		objList = append(objList, path)
	}
	return c.makeBinary(file, objList)
}

func (c *Compiler) makeObject(file *File) (path string, err error) {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	objectPath := filepath.Join(c.OutputDir, base[:dotIndex]+".o")
	cmd := exec.Command("g++", "-o", objectPath, "-c", file.Path)
	cmd.Stdout = os.Stdout
	cmd.Stdout = os.Stderr
	err = cmd.Run()
	return objectPath, err
}

func (c *Compiler) makeBinary(file *File, objectPaths []string) (path string, err error) {
	base := filepath.Base(file.Path)
	dotIndex := strings.LastIndex(base, ".")
	binaryPath := filepath.Join(c.OutputDir, base[:dotIndex])
	cmd := exec.Command("g++", "-o", binaryPath)
	cmd.Args = append(cmd.Args, objectPaths...)
	cmd.Stdout = os.Stdout
	cmd.Stdout = os.Stderr
	err = cmd.Run()
	return binaryPath, err
}
