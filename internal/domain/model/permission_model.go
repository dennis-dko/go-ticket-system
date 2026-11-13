package model

import "slices"

// Permissions are the atomic rights a role can hold. They are constants rather
// than database rows because each one is enforced by a specific line of code:
// a permission that exists only in the database would grant nothing.
const (
	PermUserRead   = "users:read"
	PermUserWrite  = "users:write"
	PermUserDelete = "users:delete"

	PermRoleRead  = "roles:read"
	PermRoleWrite = "roles:write"

	// PermSettingsManage opens the installation itself: the database connection,
	// the directory bind, the mailboxes tickets arrive through, appearance,
	// maintenance mode, telemetry, the process log and the restart.
	//
	// Granting it costs what Authorizer.RequireInstallationAdmin records: whoever
	// holds roles:write can tick it on a role and assign that role, so the two are
	// the installation between them. It exists because somebody who administers an
	// installation has to be able to do so without signing in as the built-in
	// account, which is the one account whose actions cannot be attributed to a
	// person.
	PermSettingsManage = "settings:manage"

	// PermQueueWrite sets up how tickets are sorted and answered: the queues and
	// which role reaches each, the ticket types, the service levels and the answer
	// templates.
	//
	// Whoever holds it reaches every queue. A separate lock on the queues
	// themselves would be decoration: the holder can grant their own role access to
	// any of them in one request, so the rule says what is true rather than
	// pretending otherwise.
	PermQueueWrite = "queues:write"

	// The four rights on tickets. Each reaches only the queues the holder's role
	// was given - reading where the role may read a queue, the other three where it
	// may work in it - and a ticket in a queue the role does not reach is not found
	// rather than refused; see service.TicketAccess.
	//
	// Creating is separate from working because the two are different jobs in a
	// service desk: the person at the telephone opens tickets all day and may not
	// be the one who answers them. Deleting is separate because it is the one thing
	// done to a ticket that cannot be taken back - a ticket that is finished is
	// closed, and deleting is for what should never have been one.
	PermTicketRead   = "tickets:read"
	PermTicketCreate = "tickets:create"
	PermTicketWrite  = "tickets:write"
	PermTicketDelete = "tickets:delete"

	// PermCustomerRead and PermCustomerWrite are the organisations and the contacts
	// tickets come from. They are not narrowed by queue: a customer is the same
	// customer whichever queue their ticket landed in, and an agent who could not
	// look one up could not open a ticket for them.
	PermCustomerRead  = "customers:read"
	PermCustomerWrite = "customers:write"

	// PermFAQRead and PermFAQWrite are the knowledge base as an agent has it:
	// reading includes the articles marked internal, and writing decides who may
	// read each. A customer reads the published ones with PermPortalUse instead.
	PermFAQRead  = "faq:read"
	PermFAQWrite = "faq:write"

	// PermReportRead is the figures over tickets, narrowed to the queues the role
	// may read like the tickets they count.
	PermReportRead = "reports:read"

	// PermPortalUse is the customer's side: opening tickets, reading and answering
	// their own, and the published knowledge base. It belongs to an account linked
	// to a contact, and reaches that contact's tickets and nobody else's - an
	// account holding it without such a link reaches none at all.
	PermPortalUse = "portal:use"
)

// AllPermissions lists every permission the application enforces. The role
// administration UI offers exactly these, so a typo cannot create a permission
// that no code ever checks.
func AllPermissions() []string {
	return []string{
		PermUserRead, PermUserWrite, PermUserDelete,
		PermRoleRead, PermRoleWrite,
		PermSettingsManage,
		PermQueueWrite,
		PermTicketRead, PermTicketCreate, PermTicketWrite, PermTicketDelete,
		PermCustomerRead, PermCustomerWrite,
		PermFAQRead, PermFAQWrite,
		PermReportRead,
		PermPortalUse,
	}
}

// AdminPermissions is what the built-in administrator holds: everything an
// account on the staff side can hold.
//
// Administration and the work itself, on purpose. A ticket is the
// organisation's record rather than one person's, so there is nothing in it the
// account that runs the installation must be kept from - and whoever sets up
// queues, mailboxes and templates has to be able to open a ticket to see that
// what they set up works.
//
// Not PermPortalUse. That is the customer's side, and it means something only
// for an account linked to a contact.
func AdminPermissions() []string {
	return []string{
		PermUserRead, PermUserWrite, PermUserDelete,
		PermRoleRead, PermRoleWrite,
		PermSettingsManage,
		PermQueueWrite,
		PermTicketRead, PermTicketCreate, PermTicketWrite, PermTicketDelete,
		PermCustomerRead, PermCustomerWrite,
		PermFAQRead, PermFAQWrite,
		PermReportRead,
	}
}

// AgentPermissions is what somebody who works on tickets holds: the tickets of
// the queues their role reaches, the customers, the knowledge base and the
// figures - and nothing that changes how the installation is set up.
func AgentPermissions() []string {
	return []string{
		PermTicketRead, PermTicketCreate, PermTicketWrite,
		PermCustomerRead, PermCustomerWrite,
		PermFAQRead, PermFAQWrite,
		PermReportRead,
	}
}

// CustomerPermissions is what a customer's account holds: the portal, and
// through it their own tickets.
func CustomerPermissions() []string {
	return []string{PermPortalUse}
}

// AdministeringPermissions are the rights that amount to the installation: the
// two that can change what anybody may do, and the one that opens its
// configuration. A role holding any of them is not handed to accounts nobody
// chose one by one - the directory's default role, a customer's account.
func AdministeringPermissions() []string {
	return []string{PermSettingsManage, PermRoleWrite, PermUserWrite}
}

// IsPermission reports whether name is a permission this application enforces.
func IsPermission(name string) bool {
	return slices.Contains(AllPermissions(), name)
}

// Default role names created on first start.
//
// These are identifiers, and the interface does not show them raw - each is
// translated where it is displayed, so a German reader chooses "Agent" rather
// than a lowercase English word. An installation that renamed a role sees
// whatever it typed, which is the only sensible answer for words this
// application has never seen.
const (
	RoleAdmin = "admin"

	// RoleAgent is somebody who works on tickets. It is what an account is given
	// when nobody says otherwise, and what the directory hands out by default.
	RoleAgent = "agent"

	// RoleCustomer is the role of a contact's own account, which reaches the
	// customer portal and nothing on the staff side. Shipped, and undeletable like
	// the other two, because giving a contact access to the portal has to have a
	// role to give them.
	RoleCustomer = "customer"
)

// DefaultRoles is the role set seeded on first start. Admin is marked as a
// system role: it cannot be deleted or stripped of its permissions, otherwise
// an installation could be locked out of its own administration.
func DefaultRoles() []Role {
	return []Role{
		{
			Name:        RoleAdmin,
			Description: "Administers the installation and may do everything in it",
			IsSystem:    true,
			Permissions: AdminPermissions(),
		},
		{
			Name:        RoleAgent,
			Description: "Works on the tickets of the queues the role was given",
			Permissions: AgentPermissions(),
		},
		{
			Name:        RoleCustomer,
			Description: "A customer: opens tickets in the portal and follows their own",
			Permissions: CustomerPermissions(),
		},
	}
}
