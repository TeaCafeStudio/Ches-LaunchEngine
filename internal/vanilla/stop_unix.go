//go:build !windows

package vanilla

import "os"

func stopProcess(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
