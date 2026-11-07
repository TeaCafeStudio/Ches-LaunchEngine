//go:build windows

package vanilla

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

var (
	enumWindowsProc    = syscall.NewLazyDLL("user32.dll").NewProc("EnumWindows")
	getWindowPIDProc   = syscall.NewLazyDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	sendMessageProc    = syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW")
	stopWindowMu       sync.Mutex
	stopTargetPID      uint32
	stopWindowFound    bool
	stopWindowErr      error
	stopWindowCallback = syscall.NewCallback(enumMinecraftWindow)
)

func stopProcess(process *os.Process) error {
	stopWindowMu.Lock()
	defer stopWindowMu.Unlock()
	stopTargetPID = uint32(process.Pid)
	stopWindowFound = false
	stopWindowErr = nil
	ok, _, err := enumWindowsProc.Call(stopWindowCallback, 0)
	if stopWindowErr != nil {
		return stopWindowErr
	}
	if ok == 0 {
		if err != syscall.Errno(0) {
			return err
		}
		return errors.New("enumerate Minecraft windows failed")
	}
	if !stopWindowFound {
		return errors.New("Minecraft has no window to close; use stop --force <process-id> if necessary")
	}
	return nil
}

func enumMinecraftWindow(window, _ uintptr) uintptr {
	var processID uint32
	getWindowPIDProc.Call(window, uintptr(unsafe.Pointer(&processID)))
	if processID != stopTargetPID {
		return 1
	}
	stopWindowFound = true
	var result uintptr
	sent, _, err := sendMessageProc.Call(window, 0x0010, 0, 0, 0x0002, 1000, uintptr(unsafe.Pointer(&result)))
	if sent == 0 {
		if err == syscall.Errno(0) {
			stopWindowErr = errors.New("Minecraft window did not respond to the close request")
		} else {
			stopWindowErr = err
		}
		return 0
	}
	return 1
}
