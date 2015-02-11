package cppdep

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Generator interface {
	Generate(inputFile, outputDir string) error
	Match(path string) bool
	OutputPaths(inputFile, outputDir string) []string
}

type TypeGenerator struct {
	InputExt   string
	OutputExts []string
	Command    []string
}

var genHook func(input string)

func (g *TypeGenerator) Match(path string) bool {
	return strings.HasSuffix(path, g.InputExt)
}

func outputPrefix(inputFile, outputDir string) string {
	base := filepath.Base(inputFile)
	dotIndex := strings.LastIndex(base, ".")
	if dotIndex == -1 {
		return filepath.Join(outputDir, base)
	}
	return filepath.Join(outputDir, base[:dotIndex])
}

func (g *TypeGenerator) OutputPaths(inputFile, outputDir string) []string {
	prefix := outputPrefix(inputFile, outputDir)
	var outputPaths []string
	for _, outExt := range g.OutputExts {
		outputPaths = append(outputPaths, fmt.Sprintf("%s%s", prefix, outExt))
	}
	return outputPaths
}

func (g *TypeGenerator) Generate(inputFile, outputDir string) error {
	td := map[string]string{
		"$CPPDEP_INPUT_DIR":     filepath.Dir(inputFile),
		"$CPPDEP_INPUT_FILE":    inputFile,
		"$CPPDEP_OUTPUT_DIR":    outputDir,
		"$CPPDEP_OUTPUT_PREFIX": outputPrefix(inputFile, outputDir),
	}

	var transformedArgs []string
	for _, arg := range g.Command[1:] {
		for evar, value := range td {
			i := strings.Index(arg, evar)
			if i == -1 {
				continue
			}
			arg = fmt.Sprintf("%s%s%s", arg[:i], value, arg[i+len(evar):])
		}
		transformedArgs = append(transformedArgs, arg)
	}

	if !supressLogging {
		fmt.Printf("Generating:")
		for _, fn := range g.OutputPaths(inputFile, outputDir) {
			fmt.Printf(" %s", filepath.Base(fn))
		}
		fmt.Print("\n")
	}

	cmd := exec.Command(g.Command[0])
	cmd.Args = append(cmd.Args, transformedArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
