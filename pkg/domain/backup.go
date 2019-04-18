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
	Id int64

	// Rule name
	Rule string

	// Unique container ID assigned by docker
	ContainerId string

	// Directory within master container for given backup instance
	// mounted to dumper container
	TempDirectory string

	// Directory within master container for given backup instance
	// where successfully completed backup should be moved
	//
	// deprecated
	TargetDirectory string

	// Full path to successful backup
	//
	// deprecated
	BackupDirectory string

	// Status of backup
	ExecStatus execStatus

	// Docker container exit code
	StatusCode int64

	// BackupSize of a successful backup
	BackupSize int64

	// Generation of a backup
	Generation int

	// Name of storage
	StorageName string

	// Path to successful backup archive (in temp mount)
	TempBackupFile string

	// Path to successful backup archive (in local/remote mount)
	BackupFile string

	CreatedAt  time.Time
	FinishedAt *time.Time
	DeletedAt  *time.Time
}
