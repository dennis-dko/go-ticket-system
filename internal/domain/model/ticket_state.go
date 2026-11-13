package model

// TicketState is where a ticket stands. The states are constants rather than
// rows because each of them does something: a new ticket is waiting for its
// first answer, a pending one comes back at a moment, a closed one stops owing
// anybody anything. A state that existed only in the database would be a word
// with no behaviour behind it.
type TicketState string

const (
	// StateNew is a ticket nobody has answered yet.
	StateNew TicketState = "new"

	// StateOpen is a ticket being worked on.
	StateOpen TicketState = "open"

	// StatePendingReminder is a ticket waiting for something - the customer, a
	// delivery, a date - that reminds its owner when the moment it is waiting
	// until has come.
	StatePendingReminder TicketState = "pending_reminder"

	// StatePendingClose is a ticket that closes by itself, as solved, when the
	// moment it is waiting until has come - unless the customer writes before.
	StatePendingClose TicketState = "pending_close"

	// StateClosedSuccessful and StateClosedUnsuccessful are a ticket that is
	// done, told apart by whether what was asked could be given.
	StateClosedSuccessful   TicketState = "closed_successful"
	StateClosedUnsuccessful TicketState = "closed_unsuccessful"

	// StateMerged is a ticket folded into another one, which is where it is
	// carried on. It is reached by merging and by nothing else, and is not left
	// again.
	StateMerged TicketState = "merged"
)

// FinishedStates lists the states of a ticket nobody has to do anything about
// any more.
func FinishedStates() []TicketState {
	return []TicketState{StateClosedSuccessful, StateClosedUnsuccessful, StateMerged}
}
