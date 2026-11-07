//go:build !windows && !darwin && !linux

package metadata

func currentOSVersion() string {
	return ""
}
