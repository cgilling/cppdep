package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/cgilling/cppdep"
	cli "github.com/jawher/mow.cli"
	"gopkg.in/yaml.v1"
)

type Config struct {
	Includes  []string
	Libraries map[string]string
	BuildDir  string
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
	rootCmd := cli.App("cppdep", "dependency graph and easy compiles")
	configPath := rootCmd.StringOpt("config", "", "path to yaml config")
	rootCmd.Command("build", "build binaries from a given directory", func(cmd *cli.Cmd) {
		cmd.Spec = "SRCDIR BINARY_NAME"
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

			st := &cppdep.SourceTree{
				IncludeDirs: config.Includes,
				Libraries:   config.Libraries,
				Concurrency: 4,
			}
			if err := st.ProcessDirectory(*srcDir); err != nil {
				log.Fatalf("Failed to process source directory: %s (%v)", *srcDir, err)
			}
			mainFile := st.FindSource(*binaryName + ".cc")
			if mainFile == nil {
				log.Fatalf("Unable to find source for %q", *binaryName)
			}

			c := &cppdep.Compiler{OutputDir: config.BuildDir}
			binaryPath, err := c.Compile(mainFile)
			if err != nil {
				log.Fatalf("Failed to compile main file at %s (%v)", mainFile.Path, err)
			}
			log.Printf("Compiled binary can be found at: %s", binaryPath)

		}
	})

	rootCmd.Run(os.Args)
}