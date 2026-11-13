package command

// DeleteUserCommand command to delete existing user
type DeleteUserCommand struct {
	ID uint

	// Confirmed says that whoever asked knows what the account leaves behind:
	// the tickets it owns stay, and are left without an owner.
	//
	// Without it an account that owns tickets still being worked on is refused
	// rather than removed. Nothing is lost either way - a ticket outlives its
	// owner - but a ticket nobody owns is one nobody is answering, so leaving a
	// number of them like that is a decision somebody has to make on purpose
	// rather than a side effect of a click.
	Confirmed bool
}
