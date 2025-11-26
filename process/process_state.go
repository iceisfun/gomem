package process

// ProcessState represents the state of a process
type ProcessState string

const (
	ProcessRunning    ProcessState = "R" // Running
	ProcessSleeping   ProcessState = "S" // Sleeping in an interruptible wait
	ProcessWaiting    ProcessState = "D" // Waiting in uninterruptible disk sleep
	ProcessZombie     ProcessState = "Z" // Zombie
	ProcessStopped    ProcessState = "T" // Stopped (on a signal)
	ProcessTracingStp ProcessState = "t" // Tracing stop
	ProcessPaging     ProcessState = "W" // Paging
	ProcessDead       ProcessState = "X" // Dead
	ProcessWakekill   ProcessState = "K" // Wakekill
	ProcessParked     ProcessState = "P" // Parked
)
