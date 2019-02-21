package domain

import "time"

type execStatus int

const (
	// Backup in not created
	ExecStatusNew execStatus = iota

	// Backup created in DB, but dumper container is not started
	ExecStatusCreated

	// Backup created, dumper container started
	ExecStatusStarted

	// Backup created, dumper container finished with failure (error or timeout)
	ExecStatusFailure

	// Backup created, dumper container finished, results are moved to target directory
	ExecStatusSuccess
)

var ExecStatusUnfinished = []execStatus{ExecStatusNew, ExecStatusCreated, ExecStatusStarted}

type Backup struct {
	Id int64 // identifier for DB

	Rule string

	ContainerId string // running dumper container id (e.g. 'backup-do_something-XXXX-XXXX-XXXX')

	// directory within master container for given backup instance
	// mounter to dumper container
	TempDirectory string
	TargetDirectory string
	BackupDirectory string

	// status of backup
	ExecStatus execStatus

	StatusCode int64

	CreatedAt time.Time
	DeletedAt *time.Time
}
