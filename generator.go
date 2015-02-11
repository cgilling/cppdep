package cppdep

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Generator struct {
	InputExt   string
	OutputExts []string
	Command    []string
}

var genHook func(input string)

func (g *Generator) Generate(inputFile, outputDir string) error {
	base := filepath.Base(inputFile)
	dotIndex := strings.LastIndex(base, ".")
	outputPrefex := filepath.Join(outputDir, base[:dotIndex])

	td := map[string]string{
		"$CPPDEP_INPUT_DIR":     filepath.Dir(inputFile),
		"$CPPDEP_INPUT_FILE":    inputFile,
		"$CPPDEP_OUTPUT_DIR":    outputDir,
		"$CPPDEP_OUTPUT_PREFIX": outputPrefex,
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

	cmd := exec.Command(g.Command[0])
	cmd.Args = append(cmd.Args, transformedArgs...)
	fmt.Printf("Running Generate Command: %v\n", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
