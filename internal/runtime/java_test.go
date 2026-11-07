package runtime

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestParseMajor(t *testing.T) {
	tests := []struct {
		version string
		want    int
	}{
		{version: "21.0.3", want: 21},
		{version: "1.8.0_402", want: 8},
	}
	for _, test := range tests {
		got, err := parseMajor(test.version)
		if err != nil || got != test.want {
			t.Errorf("parseMajor(%q) = %d, %v; want %d", test.version, got, err, test.want)
		}
	}
}

func TestRequireMajor(t *testing.T) {
	if err := RequireMajor(JavaInfo{Major: 21, Version: "21.0.2"}, 21); err != nil {
		t.Fatal(err)
	}
	if err := RequireMajor(JavaInfo{Major: 17, Version: "17.0.10"}, 21); err == nil {
		t.Fatal("RequireMajor() accepted an incompatible Java runtime")
	}
}

func TestRequireArchitecture(t *testing.T) {
	if err := RequireArchitecture(JavaInfo{Architecture: "amd64"}, "x86_64"); err != nil {
		t.Fatal(err)
	}
	if err := RequireArchitecture(JavaInfo{Architecture: "aarch64"}, "arm64"); err != nil {
		t.Fatal(err)
	}
	if err := RequireArchitecture(JavaInfo{Architecture: "x86"}, "amd64"); err == nil {
		t.Fatal("RequireArchitecture() accepted a mismatched runtime")
	}
}

func TestJavaVersionParsingDoesNotUseClassFileVersion(t *testing.T) {
	output := strings.Join([]string{
		"java.class.version = 65.0",
		"java.specification.version = 21",
		`openjdk version "21.0.13" 2026-10-20`,
	}, "\n")
	match := javaVersionLinePattern.FindStringSubmatch(output)
	if len(match) != 2 {
		t.Fatal("Java version output was not recognized")
	}
	major, err := parseMajor(match[1])
	if err != nil || major != 21 {
		t.Fatalf("parsed Java major = %d, %v; want 21", major, err)
	}
}

func TestDetectJavaExecutable(t *testing.T) {
	javaPath := os.Getenv("CLE_TEST_JAVA")
	if javaPath == "" {
		t.Skip("set CLE_TEST_JAVA to run the local Java integration check")
	}
	info, err := DetectJava(context.Background(), javaPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Major != 21 || info.Architecture != "amd64" {
		t.Fatalf("detected Java runtime = %+v, want Java 21 amd64", info)
	}
}
