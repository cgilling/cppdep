# cppdep
Playing Around With C++ Dependencies and simple naive compilation

## Goal
The goal is to create a dependency tree of all the c/c++ source code
in a directory and its subdirectories. After this, using a few basic
rules, then automatically being able to build all the binaries defined
within the source tree.

## Assumptions
* All base filenames are unique (i.e there will not be both an a/file.h and b/file.h)
* If a file includes file.h, then the compiled binary will need to compile file.cc
* All includes of files within the source tree use double quotes rather that angle brackets

## TODO
* Allow for linking to system libraries
* Allow for code generation steps
* Automatic detection of main files
