# cppdep
[![Build Status](https://travis-ci.org/cgilling/cppdep.svg?branch=master)](https://travis-ci.org/cgilling/cppdep)

Playing Around With C++ Dependencies and simple naive compilation

## Warning

This is still in active development. Where it is now in a working state, the config file and command line interface is likely to change in the near future.

## Goal
The goal is to create a dependency tree of all the c/c++ source code
in a directory and its subdirectories. After this, using a few basic
rules, then automatically being able to build all the binaries defined
within the source tree.

## Usage

```shell
cppdep --config [CONFIG_PATH] build [--fast] [--concurrency VALUE] [--regex] SRCDIR [BINARY_NAME]
```
* `--config`: path to the yaml config file defining the parameters for the build.
* `--fast`: Enable fast include scanning. This means that scanning a file for include statements will stop as soon as a line is found that is not a preprocessor statement, comment, or empty line. (This significantly speeds up the dependency phase – on my test machine/source tree generation time goes from 5s to 0.3s)
* `--regex`: interpret BINARY_NAME as a regular expression as defined by the [regexp package](http://golang.org/pkg/regexp/)
* `--concurrency`: maximum number of concurrent compiles. Also controls the number of files that will be concurrently scanned for dependencies. Because dependency scanning is CPU intensive, it is recommended to test and see what value of `GOMAXPROCS` is appropriate. (Somewhere between 2-4 seems about right based on my testing)

## Config

TODO

## Assumptions
* All base filenames are unique (i.e there will not be both an a/file.h and b/file.h)
* If a file includes file.h, then the compiled binary will need to compile file.cc
* All includes of files within the source tree use double quotes rather that angle brackets

## TODO
* option to automaticly include all directories in source tree in the include search path
* ability to print a list of dependencies rather than compile
