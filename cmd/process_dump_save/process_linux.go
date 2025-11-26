package main

import (
	"gomem/process"
	"gomem/process_linux"
)

func getProcess(pid int) (process.Process, error) {
	return process_linux.NewWithPID(process.ProcessID(pid))
}
