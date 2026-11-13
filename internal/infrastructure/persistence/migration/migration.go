package migration

import (
	"fmt"

	"gofr.dev/pkg/gofr/migration"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/sqldb"
)

// All returns the migration set for the given dialect, keyed by version.
//
// The dialect is passed in rather than detected because GoFr's
// migration.Datasource exposes only query methods, not the dialect, and the
// DDL for auto-increment keys and timestamps genuinely differs per engine.
//
// Every entry is one call, and what it calls runs one schema statement or
// changes rows, never both: MySQL commits each schema statement by itself, so a
// migration holding two of them, or one and a change to the data, is not rolled
// back as a unit there. That is why an index is a migration of its own rather
// than a second statement beside its table.
func All(dialect string) map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		// Who may sign in, and what each may do.
		20261112010000: {UP: func(d migration.Datasource) error { return createRoles(d, dialect) }},
		20261112010100: {UP: func(d migration.Datasource) error { return createRolePermissions(d, dialect) }},
		20261112010200: {UP: func(d migration.Datasource) error { return seedRoles(d, dialect) }},
		20261112010300: {UP: func(d migration.Datasource) error { return createUsers(d, dialect) }},
		20261112010400: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_users_external_id", "users", "external_id")
		}},
		20261112010500: {UP: func(d migration.Datasource) error { return createSessions(d, dialect) }},
		20261112010600: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_sessions_user", "sessions", "user_id")
		}},
		20261112010700: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_sessions_expiry", "sessions", "expires_at")
		}},
		20261112010800: {UP: func(d migration.Datasource) error { return createAPITokens(d, dialect) }},
		20261112010900: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_api_tokens_user", "api_tokens", "user_id")
		}},
		20261112011000: {UP: func(d migration.Datasource) error { return createPasskeys(d, dialect) }},
		20261112011100: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_passkeys_user", "passkeys", "user_id")
		}},
		20261112011200: {UP: func(d migration.Datasource) error { return createOpenIDIdentities(d, dialect) }},
		20261112011300: {UP: func(d migration.Datasource) error { return createPushSubscriptions(d, dialect) }},

		// How the installation is run.
		20261112011400: {UP: func(d migration.Datasource) error { return createSettings(d, dialect) }},
		20261112011500: {UP: func(d migration.Datasource) error { return createAdminEvents(d, dialect) }},
		20261112011600: {UP: func(d migration.Datasource) error { return createDirectoryRuns(d, dialect) }},

		// Where tickets go and how fast they are owed an answer.
		20261112011700: {UP: func(d migration.Datasource) error { return createServiceLevels(d, dialect) }},
		20261112011800: {UP: func(d migration.Datasource) error { return createQueues(d, dialect) }},
		20261112011900: {UP: func(d migration.Datasource) error { return createQueueRoles(d, dialect) }},
		20261112012000: {UP: func(d migration.Datasource) error { return createTicketTypes(d, dialect) }},
		20261112012100: {UP: func(d migration.Datasource) error { return seedTheFirstQueue(d, dialect) }},
		20261112012200: {UP: func(d migration.Datasource) error { return seedTicketTypes(d, dialect) }},
		20261112012300: {UP: func(d migration.Datasource) error { return createUserQueues(d, dialect) }},

		// Who tickets come from.
		20261112012400: {UP: func(d migration.Datasource) error { return createOrganisations(d, dialect) }},
		20261112012500: {UP: func(d migration.Datasource) error { return createContacts(d, dialect) }},
		20261112012600: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_contacts_organisation", "contacts", "organisation_id")
		}},

		// The tickets, and everything written in them.
		20261112012700: {UP: func(d migration.Datasource) error { return createTickets(d, dialect) }},
		20261112012800: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_tickets_queue_state", "tickets", "queue_id, state")
		}},
		20261112012900: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_tickets_owner", "tickets", "owner_id")
		}},
		20261112013000: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_tickets_contact", "tickets", "contact_id")
		}},
		20261112013100: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_tickets_organisation", "tickets", "organisation_id")
		}},
		20261112013200: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_tickets_updated", "tickets", "updated_at")
		}},
		20261112013300: {UP: func(d migration.Datasource) error { return createArticles(d, dialect) }},
		20261112013400: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_articles_ticket", "articles", "ticket_id")
		}},
		20261112013500: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_articles_message", "articles", "message_id")
		}},
		20261112013600: {UP: func(d migration.Datasource) error { return createAttachments(d, dialect) }},
		20261112013700: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_attachments_article", "attachments", "article_id")
		}},
		20261112013800: {UP: func(d migration.Datasource) error { return createTicketHistory(d, dialect) }},
		20261112013900: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_ticket_history_ticket", "ticket_history", "ticket_id")
		}},
		20261112014000: {UP: func(d migration.Datasource) error { return createTicketLinks(d, dialect) }},
		20261112014100: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_ticket_links_linked", "ticket_links", "linked_id")
		}},
		20261112014200: {UP: func(d migration.Datasource) error { return createTicketWatchers(d, dialect) }},
		20261112014300: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_ticket_watchers_user", "ticket_watchers", "user_id")
		}},
		20261112014400: {UP: func(d migration.Datasource) error { return createNotifications(d, dialect) }},
		20261112014500: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_notifications_user", "notifications", "user_id, read_at")
		}},

		// What an answer is written from, and how it leaves and arrives.
		20261112014600: {UP: func(d migration.Datasource) error { return createResponseTemplates(d, dialect) }},
		20261112014700: {UP: func(d migration.Datasource) error { return createMailAccounts(d, dialect) }},

		// The knowledge base.
		20261112014800: {UP: func(d migration.Datasource) error { return createFAQCategories(d, dialect) }},
		20261112014900: {UP: func(d migration.Datasource) error { return createFAQArticles(d, dialect) }},
		20261112015000: {UP: func(d migration.Datasource) error {
			return createIndex(d, "idx_faq_articles_category", "faq_articles", "category_id")
		}},
	}
}

// createRoles keeps the named sets of rights. A role is a row because an
// installation makes its own; what a right means is code, which is why the
// rights themselves are not.
func createRoles(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE roles (
		%s,
		name VARCHAR(64) NOT NULL UNIQUE,
		description VARCHAR(255) NOT NULL DEFAULT '',
		is_system %s NOT NULL DEFAULT %s
	)`, primaryKey(dialect), boolean(dialect), falseLiteral(dialect)))
}

// createRolePermissions keeps which rights each role holds, the pair being the
// key and the one index the table needs.
func createRolePermissions(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE role_permissions (
		role_id %s NOT NULL REFERENCES roles(id),
		permission VARCHAR(64) NOT NULL,
		PRIMARY KEY (role_id, permission)
	)`, foreignKeyID(dialect)))
}

// seedRoles inserts the roles an installation starts with, and their rights.
//
// From the model rather than written out: the roles shipped and the roles
// seeded are one list, and the migration that seeds them is the only one there
// has ever been, so there is no applied history for a later change to the model
// to contradict. A role added to the model after a release is a migration of
// its own, written out, at the end of this chain.
func seedRoles(d migration.Datasource, dialect string) error {
	insertRole := sqldb.Rebind(dialect,
		"INSERT INTO roles (name, description, is_system) VALUES (?, ?, ?)")

	grant := sqldb.Rebind(dialect,
		"INSERT INTO role_permissions (role_id, permission) SELECT id, ? FROM roles WHERE name = ?")

	for _, role := range model.DefaultRoles() {
		if _, err := d.SQL.Exec(insertRole, role.Name, role.Description, role.IsSystem); err != nil {
			return fmt.Errorf("seeding role %q: %w", role.Name, err)
		}

		for _, permission := range role.Permissions {
			if _, err := d.SQL.Exec(grant, permission, role.Name); err != nil {
				return fmt.Errorf("granting %q to %q: %w", permission, role.Name, err)
			}
		}
	}

	return nil
}

// createUsers keeps the accounts: the people on the staff side, and the
// customers who were given access to the portal.
//
// The second factor's column is 255 wide although a seed is 32 characters,
// because it is stored encrypted where a key is configured, and the ciphertext
// is a nonce, the seed, a tag and a marker in base64. SQLite does not enforce a
// width and the servers do, so a column cut to the plain value works everywhere
// it is tried and fails only where it matters.
//
// An empty timezone, language and theme mean "follow the installation", "the
// default" and "the time of day". Stored empty rather than resolved, so changing
// the installation's choice moves everybody who never made their own.
func createUsers(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE users (
		%[1]s,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL UNIQUE,
		role_id %[2]s NOT NULL REFERENCES roles(id),
		password_hash VARCHAR(255) NOT NULL DEFAULT '',
		must_change_password %[3]s NOT NULL DEFAULT %[4]s,
		is_system %[3]s NOT NULL DEFAULT %[4]s,
		totp_secret VARCHAR(255) NOT NULL DEFAULT '',
		totp_enabled %[3]s NOT NULL DEFAULT %[4]s,
		language VARCHAR(8) NOT NULL DEFAULT '',
		theme VARCHAR(8) NOT NULL DEFAULT '',
		timezone VARCHAR(64) NOT NULL DEFAULT '',
		tour_seen %[3]s NOT NULL DEFAULT %[4]s,
		is_external %[3]s NOT NULL DEFAULT %[4]s,
		external_id VARCHAR(128) NOT NULL DEFAULT ''
	)`, primaryKey(dialect), foreignKeyID(dialect), boolean(dialect), falseLiteral(dialect)))
}

// createSessions keeps who is signed in. The token's hash is the primary key:
// lookups happen on every request, by that and nothing else.
//
// last_seen_at is what an idle timeout measures against, and api_token_id the
// personal token a session was exchanged for, so that taking the token back
// ends the session with it. That one is no foreign key: a session goes with its
// token rather than holding it, and the erasure deletes an account's tokens
// before its sessions.
func createSessions(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE sessions (
		token_hash VARCHAR(64) PRIMARY KEY,
		user_id %[1]s NOT NULL REFERENCES users(id),
		created_at %[2]s NOT NULL,
		expires_at %[2]s NOT NULL,
		last_seen_at %[2]s,
		api_token_id %[1]s
	)`, foreignKeyID(dialect), timestamp(dialect)))
}

// createAPITokens keeps the personal tokens used for API access.
//
// A token stores only a hash, plus a short readable prefix so its owner can
// tell their tokens apart in a list without the secret being present.
func createAPITokens(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE api_tokens (
		%[1]s,
		user_id %[2]s NOT NULL REFERENCES users(id),
		name VARCHAR(120) NOT NULL,
		token_hash VARCHAR(64) NOT NULL UNIQUE,
		prefix VARCHAR(32) NOT NULL,
		created_at %[3]s NOT NULL,
		expires_at %[3]s,
		last_used_at %[3]s
	)`, primaryKey(dialect), foreignKeyID(dialect), timestamp(dialect)))
}

// createPasskeys stores the WebAuthn credentials users register for signing in.
//
// Nothing here is worth stealing: a passkey's private half never leaves the
// device, so what is kept is the public key and the counters needed to verify a
// signature.
//
// credential_id is unique across the installation because it is what a sign-in
// arrives with: the browser sends the credential, and the credential names its
// owner. That is what allows signing in without typing a username.
func createPasskeys(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE passkeys (
		%[1]s,
		user_id %[2]s NOT NULL REFERENCES users(id),
		name VARCHAR(120) NOT NULL,
		credential_id %[3]s NOT NULL UNIQUE,
		public_key %[4]s NOT NULL,
		attestation_type VARCHAR(32) NOT NULL DEFAULT '',
		transports VARCHAR(120) NOT NULL DEFAULT '',
		sign_count BIGINT NOT NULL DEFAULT 0,
		backup_eligible %[5]s NOT NULL DEFAULT %[6]s,
		backed_up %[5]s NOT NULL DEFAULT %[6]s,
		created_at %[7]s NOT NULL,
		last_used_at %[7]s
	)`, primaryKey(dialect), foreignKeyID(dialect), credentialID(dialect), blob(dialect),
		boolean(dialect), falseLiteral(dialect), timestamp(dialect)))
}

// createOpenIDIdentities keeps which account each identity at a sign-in provider
// is linked to: the provider's issuer and the subject it gives that person.
//
// One identity an account, and one account an identity - both unique, so the
// table's two lookups have their indexes from the constraints. The issuer is
// bounded at 255 like the subject the specification bounds, so that the pair
// stays inside MySQL's index key length in four-byte characters.
func createOpenIDIdentities(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE openid_identities (
		%s,
		user_id %s NOT NULL UNIQUE REFERENCES users(id),
		issuer VARCHAR(255) NOT NULL,
		subject VARCHAR(255) NOT NULL,
		created_at %s NOT NULL,
		UNIQUE (issuer, subject)
	)`, primaryKey(dialect), foreignKeyID(dialect), timestamp(dialect)))
}

// createPushSubscriptions keeps the browsers each person asked for notifications
// to be pushed to: the push service's address, the two keys a message is
// encrypted for, and the address the browser reached the installation at.
//
// The endpoint is unique, because a browser's push channel is one subscription
// and is what one is replaced and forgotten by. It is bounded at 700 so that
// MySQL can index it in four-byte characters, which a push service's address -
// some two hundred and fifty characters - comes nowhere near. Not unique per
// person: somebody with two browsers has two.
func createPushSubscriptions(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE push_subscriptions (
		%s,
		user_id %s NOT NULL REFERENCES users(id),
		endpoint VARCHAR(700) NOT NULL UNIQUE,
		p256dh VARCHAR(100) NOT NULL,
		auth VARCHAR(50) NOT NULL,
		origin VARCHAR(255) NOT NULL,
		created_at %s NOT NULL
	)`, primaryKey(dialect), foreignKeyID(dialect), timestamp(dialect)))
}

// createSettings adds the key/value table behind the administration screen.
//
// The database connection itself is deliberately *not* stored here - it would
// live in the very database it configures - and goes to a file instead.
//
// The value is the long kind of text. An uploaded logo is kept here whole, as a
// data URI of a few hundred kilobytes, and MySQL's TEXT ends at 64 of them.
func createSettings(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE settings (
		key_name VARCHAR(64) PRIMARY KEY,
		value %s NOT NULL,
		updated_at %s NOT NULL
	)`, longText(dialect), timestamp(dialect)))
}

// createAdminEvents keeps who changed how the installation is run, and when:
// accounts, roles, settings, queues, a restart.
//
// The actor and the target are ids without a foreign key, on purpose. A key would
// put the table on the erasure's list, and erasing an account would then erase the
// record of what that account did to everybody else's - the one thing a log of
// administration exists to keep. An id is never handed out twice, so once its
// account is gone it names nobody, and the names are read when the log is shown.
func createAdminEvents(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE admin_events (
		%[1]s,
		at %[2]s NOT NULL,
		actor_id %[3]s NOT NULL,
		action VARCHAR(64) NOT NULL,
		target_id %[3]s,
		detail VARCHAR(255) NOT NULL DEFAULT ''
	)`, primaryKey(dialect), timestamp(dialect), foreignKeyID(dialect)))
}

// createDirectoryRuns keeps a row for every directory run that changed
// something: when, whether somebody confirmed it against a preview, how many
// accounts it removed or added, and how many tickets were left without an owner
// by the removals.
//
// Nothing about whom, and that is the point rather than an omission. The people
// a run removed are the people the erasure exists to erase, so a table naming
// them would keep exactly what the deletion was for; the log names them, for as
// long as the log lasts.
//
// No key to users: the rows outlive the accounts they counted.
func createDirectoryRuns(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE directory_runs (
		%s,
		ran_at %s NOT NULL,
		confirmed %s NOT NULL,
		deleted INT NOT NULL,
		tickets_released INT NOT NULL,
		created INT NOT NULL
	)`, primaryKey(dialect), timestamp(dialect), boolean(dialect)))
}

// createServiceLevels keeps what a ticket is owed: how long until the first
// answer, how long between answers, how long until it is solved - each in
// minutes of the installation's service hours, and zero where that promise is
// not made.
//
// Before the queues, which name the one their tickets start with.
func createServiceLevels(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE service_levels (
		%s,
		name VARCHAR(255) NOT NULL UNIQUE,
		description VARCHAR(2000) NOT NULL DEFAULT '',
		first_response_minutes INT NOT NULL DEFAULT 0,
		update_minutes INT NOT NULL DEFAULT 0,
		solution_minutes INT NOT NULL DEFAULT 0
	)`, primaryKey(dialect)))
}

// createQueues keeps where tickets are sorted to, and what a queue decides for
// the tickets in it: the address its answers are sent from, the signature under
// them, what a new ticket's sender is told, the service level a ticket starts
// with, what an answer to a closed ticket does, and after how many minutes
// without its owner doing anything a ticket is free for somebody else again.
//
// The two texts have no default because MySQL allows none on a text column;
// they are written with every row, empty where there is nothing to say.
func createQueues(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE queues (
		%[1]s,
		name VARCHAR(255) NOT NULL UNIQUE,
		description VARCHAR(2000) NOT NULL DEFAULT '',
		email VARCHAR(255) NOT NULL DEFAULT '',
		signature %[2]s NOT NULL,
		auto_reply %[2]s NOT NULL,
		service_level_id %[3]s REFERENCES service_levels(id),
		follow_up VARCHAR(16) NOT NULL,
		unlock_minutes INT NOT NULL DEFAULT 0,
		active %[4]s NOT NULL DEFAULT %[5]s
	)`, primaryKey(dialect), longText(dialect), foreignKeyID(dialect), boolean(dialect), trueLiteral(dialect)))
}

// createQueueRoles keeps which role reaches which queue, and how far: to read
// its tickets, or to work in it. A role with no row for a queue does not reach
// it at all - the queue's tickets are not there for it.
func createQueueRoles(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE queue_roles (
		queue_id %[1]s NOT NULL REFERENCES queues(id),
		role_id %[1]s NOT NULL REFERENCES roles(id),
		access VARCHAR(8) NOT NULL,
		PRIMARY KEY (queue_id, role_id)
	)`, foreignKeyID(dialect)))
}

// createTicketTypes keeps the kinds of ticket an installation tells apart. Rows
// rather than constants, because nothing here behaves differently by type: a
// type is a word to filter and count by, and which words is the installation's
// to say.
func createTicketTypes(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE ticket_types (
		%s,
		name VARCHAR(120) NOT NULL UNIQUE
	)`, primaryKey(dialect)))
}

// seedTheFirstQueue gives a new installation one queue its agents can work in,
// so that the first ticket has somewhere to go before anybody has set anything
// up. An answer to a closed ticket opens it again, which is what somebody
// answering expects.
//
// The names are written out rather than taken from the model: a finished
// migration says what it did, and must not change with a constant that is
// allowed to.
func seedTheFirstQueue(d migration.Datasource, dialect string) error {
	insertQueue := sqldb.Rebind(dialect,
		"INSERT INTO queues (name, description, signature, auto_reply, follow_up) VALUES (?, ?, ?, ?, ?)")

	if _, err := d.SQL.Exec(insertQueue, "Service Desk",
		"Where tickets arrive until an installation sorts them further", "", "", "reopen"); err != nil {
		return fmt.Errorf("seeding the first queue: %w", err)
	}

	grant := sqldb.Rebind(dialect,
		"INSERT INTO queue_roles (queue_id, role_id, access) "+
			"SELECT q.id, r.id, ? FROM queues q, roles r WHERE q.name = ? AND r.name = ?")

	if _, err := d.SQL.Exec(grant, "write", "Service Desk", "agent"); err != nil {
		return fmt.Errorf("letting agents work in the first queue: %w", err)
	}

	return nil
}

// seedTicketTypes gives a new installation the three kinds of ticket a service
// desk most often tells apart. They are rows like any other: an installation
// renames them, removes them and adds its own.
func seedTicketTypes(d migration.Datasource, dialect string) error {
	insert := sqldb.Rebind(dialect, "INSERT INTO ticket_types (name) VALUES (?)")

	for _, name := range []string{"Incident", "Service request", "Problem"} {
		if _, err := d.SQL.Exec(insert, name); err != nil {
			return fmt.Errorf("seeding the ticket type %q: %w", name, err)
		}
	}

	return nil
}

// createUserQueues keeps the queues each agent asked to be told about: a new
// ticket in one of them is news for that agent, and one in a queue they merely
// reach is not. Both ends are keys, so the table is on the erasure's list.
func createUserQueues(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE user_queues (
		user_id %[1]s NOT NULL REFERENCES users(id),
		queue_id %[1]s NOT NULL REFERENCES queues(id),
		PRIMARY KEY (user_id, queue_id)
	)`, foreignKeyID(dialect)))
}

// createOrganisations keeps the companies tickets come from. The name is not
// unique: two customers may well be called the same, and the number an
// installation gives each is what tells them apart where it keeps one.
func createOrganisations(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE organisations (
		%s,
		name VARCHAR(255) NOT NULL,
		number VARCHAR(64) NOT NULL DEFAULT '',
		address VARCHAR(1000) NOT NULL DEFAULT '',
		url VARCHAR(255) NOT NULL DEFAULT '',
		note VARCHAR(2000) NOT NULL DEFAULT '',
		created_at %s NOT NULL
	)`, primaryKey(dialect), timestamp(dialect)))
}

// createContacts keeps the people tickets come from.
//
// The address is unique and may be absent, which is why it is nullable rather
// than empty: an arriving message finds its sender by it, so no two contacts
// may share one, and every database counts two empty strings as the same value
// where it counts two NULLs as different.
//
// user_id is the account a contact signs in to the portal with, absent for the
// many who never do. Unique, because the portal shows an account its contact's
// tickets, and an account that were two contacts would have no answer to whose.
func createContacts(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE contacts (
		%[1]s,
		organisation_id %[2]s REFERENCES organisations(id),
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) UNIQUE,
		phone VARCHAR(64) NOT NULL DEFAULT '',
		note VARCHAR(2000) NOT NULL DEFAULT '',
		user_id %[2]s UNIQUE REFERENCES users(id),
		sees_organisation %[3]s NOT NULL DEFAULT %[4]s,
		created_at %[5]s NOT NULL
	)`, primaryKey(dialect), foreignKeyID(dialect), boolean(dialect), falseLiteral(dialect), timestamp(dialect)))
}

// createTickets keeps the tickets.
//
// The number is unique and absent for the moment between a ticket's row being
// written and its number being worked out from the id that row was given - both
// inside one transaction, so nobody ever reads a ticket without one.
//
// owner_id and responsible_id are keys to accounts and still not erased with
// one: a ticket outlives the agent who owned it, and the erasure takes the
// agent's name off it rather than the ticket away. created_by is an id without a
// key for the reason the administration log's are - it says who opened a ticket
// for as long as that account exists, and names nobody afterwards.
//
// merged_into is the ticket this one was folded into, without a key: a ticket
// pointing at itself or at one deleted since is a question for the code that
// reads it, not a reason to refuse a deletion.
func createTickets(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE tickets (
		%[1]s,
		number VARCHAR(32) UNIQUE,
		title VARCHAR(255) NOT NULL,
		queue_id %[2]s NOT NULL REFERENCES queues(id),
		state VARCHAR(32) NOT NULL,
		priority INT NOT NULL,
		type_id %[2]s REFERENCES ticket_types(id),
		owner_id %[2]s REFERENCES users(id),
		responsible_id %[2]s REFERENCES users(id),
		locked %[3]s NOT NULL DEFAULT %[4]s,
		locked_at %[5]s,
		contact_id %[2]s REFERENCES contacts(id),
		organisation_id %[2]s REFERENCES organisations(id),
		service_level_id %[2]s REFERENCES service_levels(id),
		channel VARCHAR(16) NOT NULL,
		created_by %[2]s,
		created_at %[5]s NOT NULL,
		updated_at %[5]s NOT NULL,
		closed_at %[5]s,
		pending_until %[5]s,
		first_response_due %[5]s,
		first_response_at %[5]s,
		update_due %[5]s,
		solution_due %[5]s,
		merged_into %[2]s
	)`, primaryKey(dialect), foreignKeyID(dialect), boolean(dialect), falseLiteral(dialect), timestamp(dialect)))
}

// createArticles keeps what is written in a ticket: an answer, a note, a call,
// a message that arrived.
//
// author_id and contact_id are ids without a key, like a ticket's created_by: an
// article outlives whoever wrote it, and is then signed by nobody.
//
// message_id and in_reply_to are a message's own, kept so that an answer
// arriving weeks later finds the ticket it answers, and so that the same
// message fetched twice is not written twice.
func createArticles(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE articles (
		%[1]s,
		ticket_id %[2]s NOT NULL REFERENCES tickets(id),
		sender VARCHAR(16) NOT NULL,
		channel VARCHAR(16) NOT NULL,
		visible %[3]s NOT NULL,
		subject VARCHAR(255) NOT NULL DEFAULT '',
		body %[4]s NOT NULL,
		from_address VARCHAR(500) NOT NULL DEFAULT '',
		to_addresses VARCHAR(2000) NOT NULL DEFAULT '',
		cc_addresses VARCHAR(2000) NOT NULL DEFAULT '',
		message_id VARCHAR(255) NOT NULL DEFAULT '',
		in_reply_to VARCHAR(255) NOT NULL DEFAULT '',
		author_id %[2]s,
		contact_id %[2]s,
		time_minutes INT NOT NULL DEFAULT 0,
		created_at %[5]s NOT NULL
	)`, primaryKey(dialect), foreignKeyID(dialect), boolean(dialect), longText(dialect), timestamp(dialect)))
}

// createAttachments keeps the files that came with an article, in the database
// rather than beside it: the installation is one binary and one database, and a
// directory of uploads would be a third thing to back up, move and lose.
func createAttachments(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE attachments (
		%s,
		article_id %s NOT NULL REFERENCES articles(id),
		filename VARCHAR(255) NOT NULL,
		content_type VARCHAR(255) NOT NULL,
		size INT NOT NULL,
		content %s NOT NULL
	)`, primaryKey(dialect), foreignKeyID(dialect), largeBlob(dialect)))
}

// createTicketHistory keeps what happened to a ticket, a line at a time: who
// did it, what changed, from what to what. The actor is an id without a key,
// for the reason an article's author is.
func createTicketHistory(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE ticket_history (
		%[1]s,
		ticket_id %[2]s NOT NULL REFERENCES tickets(id),
		at %[3]s NOT NULL,
		actor_kind VARCHAR(16) NOT NULL,
		actor_id %[2]s,
		event VARCHAR(32) NOT NULL,
		old_value VARCHAR(255) NOT NULL DEFAULT '',
		new_value VARCHAR(255) NOT NULL DEFAULT ''
	)`, primaryKey(dialect), foreignKeyID(dialect), timestamp(dialect)))
}

// createTicketLinks keeps which tickets belong together. One row a pair, the
// lower id first, so a link is found from either end and cannot be made twice.
func createTicketLinks(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE ticket_links (
		ticket_id %[1]s NOT NULL REFERENCES tickets(id),
		linked_id %[1]s NOT NULL REFERENCES tickets(id),
		kind VARCHAR(16) NOT NULL,
		PRIMARY KEY (ticket_id, linked_id)
	)`, foreignKeyID(dialect)))
}

// createTicketWatchers keeps who asked to be told about a ticket that is not
// theirs. Erased with the account: a watch is the person's own choice, and
// means nothing without them.
func createTicketWatchers(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE ticket_watchers (
		ticket_id %[1]s NOT NULL REFERENCES tickets(id),
		user_id %[1]s NOT NULL REFERENCES users(id),
		PRIMARY KEY (ticket_id, user_id)
	)`, foreignKeyID(dialect)))
}

// createNotifications keeps what each agent was told about a ticket and whether
// they have read it. sent says whether it has left for the browsers and the
// address that asked for it, so that the job which sends them sends each once.
func createNotifications(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE notifications (
		%[1]s,
		user_id %[2]s NOT NULL REFERENCES users(id),
		ticket_id %[2]s NOT NULL REFERENCES tickets(id),
		kind VARCHAR(32) NOT NULL,
		created_at %[3]s NOT NULL,
		read_at %[3]s,
		sent %[4]s NOT NULL DEFAULT %[5]s
	)`, primaryKey(dialect), foreignKeyID(dialect), timestamp(dialect), boolean(dialect), falseLiteral(dialect)))
}

// createResponseTemplates keeps the answers written once and sent often. One
// without a queue is offered in every queue.
func createResponseTemplates(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE response_templates (
		%s,
		name VARCHAR(255) NOT NULL UNIQUE,
		body %s NOT NULL,
		queue_id %s REFERENCES queues(id)
	)`, primaryKey(dialect), longText(dialect), foreignKeyID(dialect)))
}

// createMailAccounts keeps the mailboxes tickets arrive through: where each is,
// how it is signed in to, and the queue what arrives in it goes to. The
// password is stored encrypted where a key is configured, which is why its
// column is wider than any password.
//
// last_error is what the last fetch failed on, empty after one that worked - a
// mailbox that stopped being read looks exactly like one nobody writes to,
// unless something says so.
func createMailAccounts(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE mail_accounts (
		%[1]s,
		name VARCHAR(255) NOT NULL UNIQUE,
		host VARCHAR(255) NOT NULL,
		port INT NOT NULL,
		username VARCHAR(255) NOT NULL,
		password VARCHAR(1000) NOT NULL DEFAULT '',
		security VARCHAR(16) NOT NULL,
		folder VARCHAR(255) NOT NULL,
		queue_id %[2]s NOT NULL REFERENCES queues(id),
		active %[3]s NOT NULL DEFAULT %[4]s,
		last_fetch_at %[5]s,
		last_error VARCHAR(500) NOT NULL DEFAULT ''
	)`, primaryKey(dialect), foreignKeyID(dialect), boolean(dialect), trueLiteral(dialect), timestamp(dialect)))
}

// createFAQCategories keeps the headings the knowledge base is sorted under.
func createFAQCategories(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE faq_categories (
		%s,
		name VARCHAR(255) NOT NULL UNIQUE
	)`, primaryKey(dialect)))
}

// createFAQArticles keeps the knowledge base: what each article says, and who
// may read it - the staff alone, customers who are signed in, or anybody.
// updated_by is an id without a key, like an article's author in a ticket.
func createFAQArticles(d migration.Datasource, dialect string) error {
	return exec(d, fmt.Sprintf(`CREATE TABLE faq_articles (
		%[1]s,
		category_id %[2]s REFERENCES faq_categories(id),
		title VARCHAR(255) NOT NULL,
		body %[3]s NOT NULL,
		keywords VARCHAR(255) NOT NULL DEFAULT '',
		visibility VARCHAR(16) NOT NULL,
		language VARCHAR(8) NOT NULL DEFAULT '',
		created_at %[4]s NOT NULL,
		updated_at %[4]s NOT NULL,
		updated_by %[2]s
	)`, primaryKey(dialect), foreignKeyID(dialect), longText(dialect), timestamp(dialect)))
}

// createIndex adds one index, as a migration of its own - see All for why it is
// not a second statement beside its table.
//
// The three names are written in All and nowhere else, so nothing a request
// carries reaches this statement.
func createIndex(d migration.Datasource, name, table, columns string) error {
	return exec(d, "CREATE INDEX "+name+" ON "+table+" ("+columns+")")
}

// exec runs one statement of a migration and says which one failed.
func exec(d migration.Datasource, statement string) error {
	if _, err := d.SQL.Exec(statement); err != nil {
		return fmt.Errorf("migration statement failed (%s): %w", statement, err)
	}

	return nil
}

// primaryKey returns the dialect's auto-incrementing primary key declaration.
func primaryKey(dialect string) string {
	switch dialect {
	case sqldb.DialectPostgres:
		return "id SERIAL PRIMARY KEY"
	case sqldb.DialectMySQL:
		return "id INT AUTO_INCREMENT PRIMARY KEY"
	default: // sqlite
		return "id INTEGER PRIMARY KEY AUTOINCREMENT"
	}
}

// timestamp returns the dialect's type for a point in time.
func timestamp(dialect string) string {
	switch dialect {
	case sqldb.DialectPostgres:
		return "TIMESTAMPTZ"
	case sqldb.DialectMySQL:
		return "DATETIME"
	default: // sqlite
		return "TIMESTAMP"
	}
}

// foreignKeyID returns the column type for a foreign key, which must match the
// referenced primary key's type exactly on MySQL.
func foreignKeyID(dialect string) string {
	if dialect == sqldb.DialectMySQL {
		return "INT"
	}

	return "INTEGER"
}

// boolean returns the dialect's boolean type.
func boolean(dialect string) string {
	if dialect == sqldb.DialectSQLite {
		// SQLite has no real boolean type; the declaration is documentation
		// and the values are stored as 0/1.
		return "INTEGER"
	}

	return "BOOLEAN"
}

// falseLiteral returns how the dialect spells false in a DEFAULT clause.
func falseLiteral(dialect string) string {
	if dialect == sqldb.DialectSQLite {
		return "0"
	}

	return "FALSE"
}

// trueLiteral returns how the dialect spells true in a DEFAULT clause.
func trueLiteral(dialect string) string {
	if dialect == sqldb.DialectSQLite {
		return "1"
	}

	return "TRUE"
}

// longText is the type for text with no sensible bound: what somebody wrote in
// a ticket, a message as it arrived. TEXT everywhere but MySQL, whose TEXT ends
// at 64 kilobytes - less than one long message with its quoted history.
func longText(dialect string) string {
	if dialect == sqldb.DialectMySQL {
		return "MEDIUMTEXT"
	}

	return "TEXT"
}

// credentialID is the type for a passkey's lookup key.
//
// MySQL cannot index a BLOB without a prefix length, so the identifier is
// stored as VARBINARY there - long enough for anything an authenticator
// produces, and indexable whole.
func credentialID(dialect string) string {
	if dialect == sqldb.DialectMySQL {
		return "VARBINARY(255)"
	}

	return blob(dialect)
}

// blob is the dialect's binary column type.
func blob(dialect string) string {
	if dialect == sqldb.DialectPostgres {
		return "BYTEA"
	}

	return "BLOB"
}

// largeBlob is the type for a file. MySQL's BLOB ends at 64 kilobytes, which is
// smaller than most things anybody attaches; LONGBLOB is the one without a
// bound worth naming, and the bound that matters is the one the upload is held
// to before it gets here.
func largeBlob(dialect string) string {
	if dialect == sqldb.DialectMySQL {
		return "LONGBLOB"
	}

	return blob(dialect)
}
