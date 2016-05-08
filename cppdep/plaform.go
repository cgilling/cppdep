package main

import "strings"

func MergePlatformConfig(platform string, config *Config) {
	var matchedPlatform string
	for pf := range config.Platforms {
		if strings.HasPrefix(platform, pf) && len(pf) > len(matchedPlatform) {
			matchedPlatform = pf
		}
	}
	if matchedPlatform == "" {
		return
	}
	pfConfig := config.Platforms[matchedPlatform]
	config.Excludes = append(config.Excludes, pfConfig.Excludes...)
	config.Includes = append(config.Includes, pfConfig.Includes...)
	config.Flags = append(config.Flags, pfConfig.Flags...)
	for key, val := range pfConfig.LinkLibraries {
		config.LinkLibraries[key] = val
	}
}
