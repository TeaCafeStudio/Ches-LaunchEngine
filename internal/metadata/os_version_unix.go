//go:build darwin || linux

package metadata

import (
	"os/exec"
	"strings"
)

func currentOSVersion() string {
	output, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
