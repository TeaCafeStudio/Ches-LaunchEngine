package metadata

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type RuleContext struct {
	OS       string
	Arch     string
	Version  string
	Features map[string]bool
}

func CurrentRuleContext(features map[string]bool) RuleContext {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "osx"
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "x86"
	case "arm64":
		arch = "aarch64"
	}
	return RuleContext{
		OS:       osName,
		Arch:     arch,
		Version:  currentOSVersion(),
		Features: features,
	}
}

func (context RuleContext) Allows(rules []Rule) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, rule := range rules {
		if !context.matches(rule) {
			continue
		}
		allowed = rule.Action == "allow"
	}
	return allowed
}

func (context RuleContext) matches(rule Rule) bool {
	if rule.Action != "allow" && rule.Action != "disallow" {
		return false
	}
	if rule.OS != nil {
		if rule.OS.Name != "" && rule.OS.Name != context.OS {
			return false
		}
		if rule.OS.Arch != "" {
			matched, err := regexp.MatchString(rule.OS.Arch, context.Arch)
			if err != nil || !matched {
				return false
			}
		}
		if rule.OS.Version != "" {
			matched, err := regexp.MatchString(rule.OS.Version, context.Version)
			if err != nil || !matched {
				return false
			}
		}
	}
	for feature, required := range rule.Features {
		if context.Features[feature] != required {
			return false
		}
	}
	return true
}

func (library Library) ArtifactPath() (string, bool) {
	if library.Downloads != nil && library.Downloads.Artifact != nil && library.Downloads.Artifact.Path != "" {
		return filepath.FromSlash(library.Downloads.Artifact.Path), true
	}
	parts := strings.Split(library.Name, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return "", false
	}
	group := strings.Split(parts[0], ".")
	segments := append(group, parts[1], parts[2])
	filename := parts[1] + "-" + parts[2]
	if len(parts) == 4 && parts[3] != "" {
		filename += "-" + parts[3]
	}
	filename += ".jar"
	segments = append(segments, filename)
	return filepath.Join(segments...), true
}

func (library Library) ArtifactURL(path string) string {
	if library.Downloads != nil && library.Downloads.Artifact != nil && library.Downloads.Artifact.URL != "" {
		return library.Downloads.Artifact.URL
	}
	base := library.URL
	if base == "" {
		base = "https://libraries.minecraft.net/"
	}
	return strings.TrimRight(base, "/") + "/" + filepath.ToSlash(path)
}

func (library Library) IsNative(path string) bool {
	parts := strings.Split(library.Name, ":")
	if len(parts) == 4 && strings.Contains(strings.ToLower(parts[3]), "native") {
		return true
	}
	return strings.Contains(strings.ToLower(filepath.Base(path)), "natives")
}
