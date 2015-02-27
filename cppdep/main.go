package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/cgilling/cppdep"
	cli "github.com/jawher/mow.cli"
	"gopkg.in/yaml.v1"
)

type Config struct {
	AutoInclude     bool
	Includes        []string
	Excludes        []string
	Libraries       map[string][]string
	SourceLibs      map[string][]string
	Flags           []string
	Binary          BinaryConfig
	Modes           map[string]ModeConfig
	SrcDir          string
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

type ModeConfig struct {
	Flags []string
}

type BinaryConfig struct {
	Rename []cppdep.RenameRule
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

func searchForConfigFile(dir string) string {
	var path string
	for dir != "/" && path == "" {
		testpath := filepath.Join(dir, "cppdep.yml")
		if _, err := os.Stat(testpath); err == nil {
			path = testpath
			break
		}
		dir = filepath.Dir(dir)
	}
	return path
}

func main() {
	makeCommandAndRun(os.Args)
}

func makeCommandAndRun(args []string) {
	cmd := cli.App("cppdep", "dependency graph and easy compiles")
	cmd.Spec = "[OPTIONS] [BINARY_NAMES]..."
	configPath := cmd.StringOpt("config", "", "path to yaml config")
	concurrency := cmd.IntOpt("c concurrency", 1, "How much concurrency to we want to allow")
	mode := cmd.StringOpt("mode", "default", "select a build mode")
	fast := cmd.BoolOpt("fast", false, "Set to enable fast file scanning")
	list := cmd.BoolOpt("list", false, "Lists paths of all binaries that would be generated, but does not compile them")
	cpuprofile := cmd.StringOpt("cpuprof", "", "file to write the cpu profile to")
	srcDir := cmd.StringOpt("src", "", "path to the src directory")
	binaryNames := cmd.StringsArg(
		"BINARY_NAMES",
		nil,
		"name of the binary to build, main source file should be BINARY_NAME.cc, this can be a globbing expression as well."+
			" A '*' on its own means 'all autodetected main source files'",
	)

	cmd.Action = func() {
		if *cpuprofile != "" {
			f, err := os.Create(*cpuprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		config := &Config{}
		if *configPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get cwd: %v", err)
			}
			*configPath = searchForConfigFile(cwd)
		}

		if *configPath == "" {
			log.Fatalf("No config file provided and no cppdep.yml found in path")
		}

		if err := config.ReadFile(*configPath); err != nil {
			log.Fatalf("Failed to read config file: %q", err)
		}

		if config.BuildDir == "" {
			log.Fatalf("BuildDir must be set")
		}

		if !filepath.IsAbs(config.BuildDir) {
			config.BuildDir = filepath.Join(filepath.Dir(*configPath), config.BuildDir)
		}

		err := os.MkdirAll(config.BuildDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create build dir: %s (%v)", config.BuildDir, err)
		}

		if *srcDir == "" && config.SrcDir == "" {
			log.Fatalf("a source directory must be set through --src or config.srcdir")
		} else if *srcDir == "" {
			if filepath.IsAbs(config.SrcDir) {
				*srcDir = config.SrcDir
			} else {
				*srcDir = filepath.Join(filepath.Dir(*configPath), config.SrcDir)
			}
		}

		if config.Modes == nil {
			config.Modes = make(map[string]ModeConfig)
		}

		if _, ok := config.Modes["default"]; !ok {
			config.Modes["default"] = ModeConfig{}
		}

		if _, ok := config.Modes[*mode]; !ok {
			log.Fatalf("Cannot find requested mode %q", *mode)
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
			AutoInclude:     config.AutoInclude,
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

		if err := st.Rename(config.Binary.Rename); err != nil {
			log.Fatalf("Failed to rename files: %v", err)
		}

		flags := make([]string, len(config.Flags))
		copy(flags, config.Flags)
		flags = append(flags, config.Modes[*mode].Flags...)

		c := &cppdep.Compiler{
			OutputDir:   filepath.Join(config.BuildDir, *mode),
			IncludeDirs: st.IncludeDirs,
			Flags:       flags,
			Concurrency: *concurrency,
		}
		var files []*cppdep.File
		if *binaryNames == nil {
			*binaryNames = []string{"*"}
		}

		for _, binaryName := range *binaryNames {
			if binaryName == "*" {
				files, err = st.FindMainFiles()
				if err != nil {
					log.Fatalf("failes to automatically find main files: %v", err)
				}
			} else {
				f, err := st.FindSources(binaryName)
				if err != nil {
					log.Fatalf("invalid pattern: %q", binaryName)
				}
				files = append(files, f...)
			}
		}

		if *list {
			for _, file := range files {
				fmt.Println(c.BinPath(file))
			}
		} else {
			binPaths, err := c.CompileAll(files)
			if err != nil {
				log.Fatalf("Compile returned error: %v", err)
			}
			binDir := filepath.Join(config.BuildDir, "bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				log.Fatalf("Failed to make directory: %q (%v)", binDir, err)
			}
			for _, path := range binPaths {
				symPath := filepath.Join(binDir, filepath.Base(path))
				if _, err := os.Lstat(symPath); err == nil {
					if err := os.Remove(symPath); err != nil {
						log.Fatalf("Failed to remove old symlink: %v", err)
					}
				}
				if err := os.Symlink(path, symPath); err != nil {
					log.Fatalf("Failed to symlink file: %v", err)
				}
			}
		}
	}

	cmd.Run(args)
}
