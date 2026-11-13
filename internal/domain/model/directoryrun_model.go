package model

import "time"

// DirectoryRun is what one directory synchronisation changed, and nothing about
// whom.
//
// A run deletes the accounts the directory no longer holds, and the only record
// of it used to be lines in the log - which a log level above WARN drops, and
// which the console's rotation ends anyway. This is the record that stays. It
// carries no name, address or identifier on purpose: the people a run removed
// are the people the erasure exists to erase, and keeping who they were would
// undo exactly that. The log still names them, for as long as it lasts.
type DirectoryRun struct {
	ID    uint
	RanAt time.Time

	// Confirmed is a run somebody started against a preview they had read;
	// false is the schedule, or a script that sent no list.
	Confirmed bool

	// Deleted counts the accounts removed, TicketsReleased the tickets those
	// accounts owned and that were left without an owner, and Created the
	// accounts added.
	Deleted         int
	TicketsReleased int
	Created         int
}
