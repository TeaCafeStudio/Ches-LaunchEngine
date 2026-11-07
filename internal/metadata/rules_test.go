package metadata

import (
	"path/filepath"
	"testing"
)

func TestRulesPlatformFeaturesAndLastMatchingAction(t *testing.T) {
	platform := RuleContext{
		OS: "windows", Arch: "x86_64", Version: "10.0",
		Features: map[string]bool{"has_custom_resolution": true},
	}
	rules := []Rule{
		{Action: "allow", OS: &OSRule{Name: "windows"}},
		{Action: "disallow", OS: &OSRule{Name: "windows"}},
		{Action: "allow", OS: &OSRule{Name: "windows", Arch: "^x86_64$"}, Features: map[string]bool{"has_custom_resolution": true}},
		{Action: "disallow", Features: map[string]bool{"is_demo_user": true}},
	}
	if !platform.Allows(rules) {
		t.Fatal("platform should match the last applicable allow rule")
	}
	platform.Features["has_custom_resolution"] = false
	if platform.Allows(rules) {
		t.Fatal("platform should not match an allow rule requiring custom resolution")
	}
}

func TestLibraryMavenPathAndNativeDetection(t *testing.T) {
	library := Library{Name: "org.lwjgl:lwjgl:3.3.3:natives-windows"}
	path, ok := library.ArtifactPath()
	if !ok || path != filepath.FromSlash("org/lwjgl/lwjgl/3.3.3/lwjgl-3.3.3-natives-windows.jar") {
		t.Fatalf("ArtifactPath() = %q, %v", path, ok)
	}
	if !library.IsNative(path) {
		t.Fatal("native classifier was not identified")
	}
}
