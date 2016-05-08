package main

import (
	"reflect"
	"sort"
	"testing"

	"gopkg.in/yaml.v2"
)

var configYAML = `
srcdir: src
builddir: build
excludes: ["exclude_base"]
includes: ["include_base"]
linklibraries:
  "base.h": ["-lbase"]
flags: ["-DBASE"]
platforms:
  myplatform:
    excludes: ["exclude_myplatform"]
    includes: ["include_myplatform"]
    linklibraries:
      "platform.h": ["-lmyplatform"]
    flags: ["-DMYPLATFORM"]
  myplatform-2:
    excludes: ["exclude_myplatform2"]
    includes: ["include_myplatform2"]
    linklibraries:
      "base.h": ["-lcustomBase"]
      "platform2.h": ["-lmyplatform2"]
    flags: ["-DMYPLATFORM2"]
  other:
    myplatform:
    excludes: ["exclude_other"]
    includes: ["include_other"]
    linklibraries:
      "other.h": ["-lother"]
    flags: ["-DOTHER"]
`

func seteq(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	sort.Sort(sort.StringSlice(s1))
	sort.Sort(sort.StringSlice(s2))
	return reflect.DeepEqual(s1, s2)
}

func TestMergePlatformConfigNoMatch(t *testing.T) {
	var conf Config
	err := yaml.Unmarshal([]byte(configYAML), &conf)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	MergePlatformConfig("notfound", &conf)

	expExcludes := []string{"exclude_base"}
	expIncludes := []string{"include_base"}
	expFlags := []string{"-DBASE"}
	expLinkLibs := map[string][]string{"base.h": {"-lbase"}}

	if exp, got := expExcludes, conf.Excludes; !seteq(exp, got) {
		t.Errorf("excludes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expIncludes, conf.Includes; !seteq(exp, got) {
		t.Errorf("includes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expFlags, conf.Flags; !seteq(exp, got) {
		t.Errorf("flags not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expLinkLibs, conf.LinkLibraries; !reflect.DeepEqual(exp, got) {
		t.Errorf("link libs not as expected:\nexp: %v\ngot: %v", exp, got)
	}
}

func TestMergePlatformConfigFullVersion(t *testing.T) {
	var conf Config
	err := yaml.Unmarshal([]byte(configYAML), &conf)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	MergePlatformConfig("myplatform-2", &conf)

	expExcludes := []string{"exclude_base", "exclude_myplatform2"}
	expIncludes := []string{"include_base", "include_myplatform2"}
	expFlags := []string{"-DBASE", "-DMYPLATFORM2"}
	expLinkLibs := map[string][]string{
		"base.h":      {"-lcustomBase"},
		"platform2.h": {"-lmyplatform2"},
	}

	if exp, got := expExcludes, conf.Excludes; !seteq(exp, got) {
		t.Errorf("excludes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expIncludes, conf.Includes; !seteq(exp, got) {
		t.Errorf("includes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expFlags, conf.Flags; !seteq(exp, got) {
		t.Errorf("flags not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expLinkLibs, conf.LinkLibraries; !reflect.DeepEqual(exp, got) {
		t.Errorf("link libs not as expected:\nexp: %v\ngot: %v", exp, got)
	}
}

func TestMergePlatformConfigPartialMatch(t *testing.T) {
	var conf Config
	err := yaml.Unmarshal([]byte(configYAML), &conf)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	MergePlatformConfig("myplatform-1", &conf)

	expExcludes := []string{"exclude_base", "exclude_myplatform"}
	expIncludes := []string{"include_base", "include_myplatform"}
	expFlags := []string{"-DBASE", "-DMYPLATFORM"}
	expLinkLibs := map[string][]string{
		"base.h":     {"-lbase"},
		"platform.h": {"-lmyplatform"},
	}

	if exp, got := expExcludes, conf.Excludes; !seteq(exp, got) {
		t.Errorf("excludes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expIncludes, conf.Includes; !seteq(exp, got) {
		t.Errorf("includes not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expFlags, conf.Flags; !seteq(exp, got) {
		t.Errorf("flags not as expected:\nexp: %v\ngot: %v", exp, got)
	}
	if exp, got := expLinkLibs, conf.LinkLibraries; !reflect.DeepEqual(exp, got) {
		t.Errorf("link libs not as expected:\nexp: %v\ngot: %v", exp, got)
	}
}
