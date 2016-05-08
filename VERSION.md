## Version Notes

### 1.2

- Changed build subdirectory hierarchy to include the platform at top level. This change was intented to allow for using Docker to build for multiple plaform versions on the same machine by mounting the source directory into the Docker container
and building there for each platform.

### 1.1

- Introduced `--version` flag to `cppdep` binary
- Added `platforms` key to config with support for `includes`, `excludes`, `linklibraries` and `flags`
- Added `--platform` flag to `cppdep` binary