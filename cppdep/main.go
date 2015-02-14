package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/cgilling/cppdep"
	cli "github.com/jawher/mow.cli"
	"gopkg.in/yaml.v1"
)

type Config struct {
	Includes        []string
	Excludes        []string
	Libraries       map[string][]string
	SourceLibs      map[string][]string
	Flags           []string
	BuildDir        string
	TypeGenerators  []TypeGeneratorConfig
	ShellGenerators []ShellGeneratorConfig
}

type TypeGeneratorConfig struct {
	InputExt   string
	OutputExts []string
	Command    []string
}

type ShellGeneratorConfig struct {
	InputPaths  []string
	OutputFiles []string
	Path        string
}

func (c *Config) ReadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	confBuf, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(confBuf, &c); err != nil {
		return err
	}
	return nil
}

func main() {
	cmd := cli.App("cppdep", "dependency graph and easy compiles")
	cmd.Spec = "[OPTIONS] SRCDIR [BINARY_NAME]"
	configPath := cmd.StringOpt("config", "", "path to yaml config")
	concurrency := cmd.IntOpt("c concurrency", 1, "How much concurrency to we want to allow")
	fast := cmd.BoolOpt("fast", false, "Set to enable fast file scanning")
	regex := cmd.BoolOpt("regex", false, "Treat binaryName as a path regex")
	srcDir := cmd.StringArg("SRCDIR", "./", "")
	binaryName := cmd.StringArg("BINARY_NAME", "", "name of the binary to build, main source file should be BINARY_NAME.cc")

	cmd.Action = func() {
		fmt.Printf("config: %q, srcDir: %q\n", *configPath, *srcDir)
		config := &Config{}
		if *configPath != "" {
			if err := config.ReadFile(*configPath); err != nil {
				log.Fatalf("Failed to read config file: %q", err)
			}
		}
		if config.BuildDir == "" {
			log.Fatalf("BuildDir must be set")
		}
		err := os.MkdirAll(config.BuildDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create build dir: %s (%v)", config.BuildDir, err)
		}

		var gens []cppdep.Generator
		for _, gen := range config.TypeGenerators {
			gens = append(gens, &cppdep.TypeGenerator{
				InputExt:   gen.InputExt,
				OutputExts: gen.OutputExts,
				Command:    gen.Command,
			})
		}
		for _, gen := range config.ShellGenerators {
			gens = append(gens, &cppdep.ShellGenerator{
				InputPaths:    gen.InputPaths,
				OutputFiles:   gen.OutputFiles,
				ShellFilePath: filepath.Join(*srcDir, gen.Path),
			})
		}

		if config.BuildDir, err = filepath.Abs(config.BuildDir); err != nil {
			log.Fatalf("Failed to get absolute path of build dir")
		}

		st := &cppdep.SourceTree{
			SrcRoot:         *srcDir,
			IncludeDirs:     config.Includes,
			ExcludeDirs:     config.Excludes,
			Libraries:       config.Libraries,
			SourceLibs:      config.SourceLibs,
			Concurrency:     *concurrency,
			UseFastScanning: *fast,
			Generators:      gens,
			BuildDir:        config.BuildDir,
		}
		if err := st.ProcessDirectory(); err != nil {
			log.Fatalf("Failed to process source directory: %s (%v)", *srcDir, err)
		}

		c := &cppdep.Compiler{
			OutputDir:   config.BuildDir,
			IncludeDirs: append(config.Includes, st.GenDir()),
			Flags:       config.Flags,
			Concurrency: *concurrency,
		}
		var files []*cppdep.File
		if *binaryName == "" {
			files, err = st.FindMainFiles()
			if err != nil {
				log.Fatalf("failes to automatically find main files: %v", err)
			}
		} else if *regex {
			files, err = st.FindSources(*binaryName + ".cc")
			if err != nil {
				log.Fatalf("invalid regex: %q", *binaryName)
			}
		} else {
			mainFile := st.FindSource(*binaryName + ".cc")
			if mainFile == nil {
				log.Fatalf("Unable to find source for %q", *binaryName)
			}
			files = append(files, mainFile)
		}

		c.CompileAll(files)
	}

	cmd.Run(os.Args)
}
