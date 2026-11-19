package model

import "strings"

// The number of backups a plan keeps: at least one, and a bound past which the
// directory is a second database rather than a set of backups.
const (
	DefaultBackupsKept = 14
	MaxBackupsKept     = 365
)

// BackupPlan is when backups are made on their own and how many are kept.
type BackupPlan struct {
	// Schedule is a cron expression in the installation's timezone. Empty makes
	// no backup on its own; a backup can still be made by hand.
	Schedule string `json:"schedule"`

	// Keep is how many backups the folder holds: after each one is made, on
	// schedule or by hand, the oldest go until this many are left.
	Keep int `json:"keep"`
}

// Normalized trims the schedule and gives a plan that names no number the
// default one.
func (p BackupPlan) Normalized() BackupPlan {
	p.Schedule = strings.TrimSpace(p.Schedule)

	if p.Keep == 0 {
		p.Keep = DefaultBackupsKept
	}

	return p
}

// Valid reports whether the plan can be kept: a schedule the scheduler reads,
// and a number of backups between one and MaxBackupsKept.
func (p BackupPlan) Valid() bool {
	return ValidSchedule(p.Schedule) && p.Keep >= 1 && p.Keep <= MaxBackupsKept
}
