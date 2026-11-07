package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

type JavaInfo struct {
	Path         string
	Version      string
	Major        int
	Architecture string
}

var javaVersionLinePattern = regexp.MustCompile(`(?im)^\s*(?:openjdk|java)\s+version\s+"([^"]+)"`)
var javaVersionPropertyPattern = regexp.MustCompile(`(?m)^\s*java\.version\s*=\s*(\S+)\s*$`)
var javaArchPattern = regexp.MustCompile(`(?m)^\s*os\.arch\s*=\s*(\S+)\s*$`)

func DetectJava(ctx context.Context, requestedPath string) (JavaInfo, error) {
	javaPath := requestedPath
	if javaPath == "" {
		found, err := exec.LookPath("java")
		if err != nil {
			return JavaInfo{}, cleerrors.Wrap(cleerrors.CodeJavaNotFound, "Java executable was not found on PATH", err)
		}
		javaPath = found
	} else if info, err := os.Stat(javaPath); err == nil && info.IsDir() {
		name := "java"
		if runtime.GOOS == "windows" {
			name = "java.exe"
		}
		javaPath = filepath.Join(javaPath, "bin", name)
	}

	command := exec.CommandContext(ctx, javaPath, "-XshowSettings:properties", "-version")
	output, err := command.CombinedOutput()
	if err != nil {
		return JavaInfo{}, cleerrors.Wrap(cleerrors.CodeJavaNotFound, "failed to execute Java", err)
	}
	match := javaVersionLinePattern.FindStringSubmatch(string(output))
	if len(match) != 2 {
		match = javaVersionPropertyPattern.FindStringSubmatch(string(output))
	}
	if len(match) != 2 {
		return JavaInfo{}, cleerrors.New(cleerrors.CodeJavaNotFound, "could not parse Java version output")
	}
	version := match[1]
	major, err := parseMajor(version)
	if err != nil {
		return JavaInfo{}, cleerrors.Wrap(cleerrors.CodeJavaNotFound, "could not parse Java major version", err)
	}
	outputText := strings.ToLower(string(output))
	architecture := runtime.GOARCH
	if archMatch := javaArchPattern.FindStringSubmatch(string(output)); len(archMatch) == 2 {
		architecture = archMatch[1]
	} else if strings.Contains(outputText, "64-bit") {
		architecture = "64-bit"
	} else if strings.Contains(outputText, "32-bit") {
		architecture = "32-bit"
	}
	return JavaInfo{
		Path: javaPath, Version: version, Major: major, Architecture: architecture,
	}, nil
}

func parseMajor(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) > 1 && parts[0] == "1" {
		return strconv.Atoi(parts[1])
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid Java version %q: %w", version, err)
	}
	return major, nil
}

func RequireMajor(info JavaInfo, required int) error {
	if required <= 0 || info.Major == required {
		return nil
	}
	return cleerrors.New(cleerrors.CodeJavaVersionIncompatible,
		fmt.Sprintf("Minecraft requires Java %d, but Java %s was detected", required, info.Version))
}

func RequireArchitecture(info JavaInfo, required string) error {
	if info.Architecture == "" || info.Architecture == "64-bit" || info.Architecture == "32-bit" {
		return nil
	}
	normalize := func(arch string) string {
		switch strings.ToLower(arch) {
		case "amd64", "x86_64":
			return "amd64"
		case "386", "x86", "i386", "i686":
			return "386"
		case "arm64", "aarch64":
			return "arm64"
		case "arm":
			return "arm"
		default:
			return strings.ToLower(arch)
		}
	}
	if normalize(info.Architecture) == normalize(required) {
		return nil
	}
	return cleerrors.New(cleerrors.CodeJavaVersionIncompatible,
		fmt.Sprintf("Java architecture %s does not match the host architecture %s", info.Architecture, required))
}
