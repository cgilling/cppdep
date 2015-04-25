# cppdep
[![Build Status](https://travis-ci.org/cgilling/cppdep.svg?branch=master)](https://travis-ci.org/cgilling/cppdep)

Simple, fast automatic dependency discovery and compilation of an arbitrary C++ source tree.

## Goal
Given an arbitrary source tree, `cppdep` is able to scan dependencies and determine which files need to be compiled into binaries. With simple configuration installed libraries can be linked against as well.

## Assumptions
In order to automatically determine all source files needed to compile a binary, the following assumptions are made about the source tree.
* All base filenames are unique (i.e there will not be both an a/file.h and b/file.h)
* If a file includes file.h, then the compiled binary will also need file.cc
* All includes of files contained within the source tree use double quotes rather that angle brackets
* All header files have one of the following extensions: `.h`, `.hpp`, `.hh`, `.hxx`
* All source files have one of the following extensions: `.cc`, `.cxx`, `.c`

## Usage

```shell
cppdep [--config CONFIG_PATH] [--fast] [--concurrency|-c VALUE] [BINARY_NAME]*
```
* `--config`: path to the yaml config file defining the parameters for the build. If not provided $CWD and all parent directories in order will be seaches for a cppdep.yml file.
* `--fast`: Enable fast include scanning. This means that scanning a file for include statements will stop as soon as a line is found that is not a preprocessor statement, comment, or empty line. (Speeds up dependency phase by over 90% on typical projects)
* `--concurrency`: maximum number of concurrent compiles. Also controls the number of files that will be concurrently scanned for dependencies.
* `BINARY_NAME`: one or more names of binaries to be compiled. If a `/` is present in the binary name it is assumed to be a relative path from the root of the `src` dir. A binary name is either the name of a `c++` source file with its extension removed, or one that has been renamed using `binary.rename` config entry. Wildcards provided in the [filepath.Match](http://golang.org/pkg/path/filepath/#Match) can be used as well to match multiple binaries. It should be noted that when specifying binary names any file that matches the given pattern will be compiled as if it were the main file of a binary (so be careful when using the `*` wildcard). If no names are provided or if a name is `*` alone, then `cppdep` will attempt to find all files that have main definitions in them and compile them all as binaries.

### Examples
Automatically detect and compile a binaries in the source tree:
```bash
cppdep --fast -c 24
```

Compile a single binary in the source tree:
```bash
cppdep --fast -c 8 myApp
```

Compile all source files starting with a given prefix as binaries:
```bash
cppdep --fast -c 24 test/gtest*
```
## Config
The config is a `YAML` file with the keys:
* **srcdir** `string` - path to the root of the source tree (relative to the directory of the config file)
* **builddir** `string` - path to the directory in which to place all build files (relative to the directory of the config file)
* **autoinclude** `bool` - if true, all directories in the source tree will be added to the compile with the `-I` flag.
* **excludes** `array of strings`: paths of directories to be exclude when scanning the source tree, relative to the root of the src tree.
* **includes** `array of strings` - include paths to be added to the compile with the `-I` flag. If `autoinclude` is not set to true, then relative paths in this list will be the only ones searched when looking for dependencies (other than the current directory of the file where the include statement is found)
* **flags** `array of strings` - a list of flags to be passed to the compiler
* **modes** `dictionary of string -> mode config dictionary` - maps a mode name to a change in configuration when compiling under that mode. Currently the only key in the mode config dictionary supported is `flags`. For example a debug mode could be defined as `modes: {debug: {flags: ["-g", "-O0"]}}`.
* **linklibraries** `dictionary of string -> array of strings` - The keys of the dictionary are includes found within angle bracken includes, and the values are the compiler statements needed to link against the appropriate library. For example if a file has `#include <uuid/uuid.h>` then the config statement containing `"uuid/uuid.h": ["-luuid"]` in the `linklibraries` section will gaurantee that any binary that needs to link against libuuid will do so.
* **libraries** `dictionary of string -> LibraryConfig` -- Maps the name of a shared library to be created to configuration on how to build it. Currently `LibraryConfig` only has a single key `sources` which is an array of relative paths (relative to srdir) of all source files which should be included in generating a shared library. All dependencies and linklibraries will be pull in and linked against as a normal binary compilation. **For example** if we wanted to compile all `mylib/a.cc` and `mylib/b.cc` into a shared library called `mylib.so` we would do `libraries: {libseu: {sources: ["mylib/a.cc", "mylib/b.cc"] } }`. Note that `libraries` are not compiled as part of the default compile or using the single `*` as a binary name. The resulting library will be named `[libname].so`.
* **sourcelibs** `dictionary of string -> array of strings` -- Maps a header include value to a list of source files to be linked against if that header is included. This is intented to be used if you have one header file in your source tree that is implemented by multiple source files. **For example** if you include [gmock](https://code.google.com/p/googlemock/) in your source tree and want binaries that include `gmock/gmock.h` to link against `gmock-gtest-all.cc` and `gmock_main.cc` you would include the following in the config: `sourcelibs: {"gmock/fused-src/gmock/gmock.h": ["gmock/fused-src/gmock-gtest-all.cc", "gmock/fused-src/gmock_main.cc"]}`.
* **binary** `dictionary of subcommand string -> subcommand config` - currently the only subcommand supported is `rename` and its config is as follows `{regex: "string", replace: "string"}`. This is used for renaming binaries which one does not want to follow the pattern of being named as the file containing the main statement minus the extension. The two arguments follow the rules as described by the [golang regexp package](http://golang.org/pkg/regexp/), for example if you wanted all files that end in Main to not contain main in the binary name you could provide the following in the config `binary: {rename: [{regex: "(.*)Main", replace: "$1"}]}`.
* **typegenerators** `array of type generator configs`: see generator section for more details
* **shellgenerators** `array of shell generator configs`: see generator section for more details

## Generators

Generators are used to creates source files from some other resource file. For example take `.proto` files and run `protoc` on them to generate their respectice `.h` and `.cc` files. Or running a custom script to create a lookup table from a text file.

### Type Generators
A type generator instructs `cppdep` how to tranform a filetype into other files that will be used as part of the compile process. Each type generator config requires that three config parameters be set:

* `inputext` : `string` -- the extension of the filetype on which to use this generator. This should include the `.`.
* `outputexts`: `array of string` -- the extension(s) of the resulting file(s) from the generations process.
* `command`: `array of string` -- a command and arguments to be run in order to to transform the input file into output files.

#### Further Details 
This command is not run in a shell so environment variables will not be available except for a few special variables that will be substituted by `cppdep` itself. The variables provided are as follows:
* `$CPPDEP_INPUT_DIR`: the path to the directory in which the input file resides
* `$CPPDEP_OUTPUT_DIR`: the path to the directory in which output files are expected to be written
* `$CPPDEP_INPUT_FILE`: the path to the file which should be tranformed by this generator command
* `$CPPDEP_OUTPUT_PREFIX`: the path to the output file minus the file extension. This is `$CPPDEP_OUTPUT_DIR` followed by the base of the input file with its extension removed.

Output files are expected to be written to `$CPPDEP_OUTPUT_PREFIX` followed by their extensions. If after running the command the expected files are not in the output directory, it is considered a failure.

### Shell Generators
Shell generators are a much more explicit version of generators. Input files and output files are explicitly defined and the generation process is controlled by running a single shell script. There are three configuration keys to be set:

* `inputpaths`: `array of string` -- relative paths (relative to srcdir) to all files that will be used to create the output files
* `outputfiles`: `array of string` -- filenames of all the file that will be created by this generator
* `path`: `string` -- relative path to shell script

#### Further Details
The environment variable defined in the Type Generators section are all available for use within the shell script, although in practice only `$CPPDEP_OUTPUT_DIR` will be useful. All output files are expected to be written to `$CPPDEP_OUTPUT_DIR`.

It is guaranteed that the shell script will be executed from the directory in which it resides, so input files can be referred to more easily with known relative paths.

