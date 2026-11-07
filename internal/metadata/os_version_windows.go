//go:build windows

package metadata

import (
	"fmt"
	"syscall"
	"unsafe"
)

type rtlOSVersionInfo struct {
	Size        uint32
	Major       uint32
	Minor       uint32
	Build       uint32
	PlatformID  uint32
	ServicePack [128]uint16
}

func currentOSVersion() string {
	procedure := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
	version := rtlOSVersionInfo{Size: uint32(unsafe.Sizeof(rtlOSVersionInfo{}))}
	result, _, _ := procedure.Call(uintptr(unsafe.Pointer(&version)))
	if result != 0 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Build)
}
