package main

import (
	"fmt"
	"gomem/process"
)

func getProcess(pid int) (process.Process, error) {
	return nil, fmt.Errorf("windows not supported on this build")
}
