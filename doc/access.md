# Accounts, roles and sign-in

Who may do what, the ways somebody signs in — a password, a passkey, a
directory, an organisation's provider — and how a script reaches the API.

## Access control

Users sign in with their email address and password. What someone may do comes
from their **role**, and roles are administered at run time through the
interface: create them, set their permissions, delete them.

| Role | Purpose |
| --- | --- |
| `admin` | Administers the installation and may do everything on the staff side, tickets included. A system role: it cannot be deleted |
| `agent` | Works on tickets, contacts and the knowledge base, and changes nothing about how the installation is set up. What an account is given when nobody says otherwise, and what the directory hands out by default |
| `customer` | A contact's own account: opens tickets in the customer portal and follows their own, and reaches nothing on the staff side |

What you see is not the identifier. Those names are lowercase, hyphenated and English
because that is what the API takes, what the directory configuration stores and what the
role editor edits — and none of that is something to put in front of somebody deciding
what a colleague may do. Each role is shown by a translated title with its description
beside it, so a German reader chooses *Kunde* rather than `customer`. The screen that
administers roles keeps the identifier visible next to the title, because that screen is
where it matters. A role an installation named itself shows whatever was typed, which is
the only sensible answer for words this application has never seen.

Permissions are fine grained — reading separately from writing, tickets separately
from contacts and from the knowledge base — and they are Go constants, not database
rows: each one is checked by a specific line of code, and a permission that existed
only in the database would grant nothing. The role editor therefore offers exactly
the permissions that are actually enforced.

One of them is the whole administration: `settings:manage` opens the *Settings*
screen and everything on it — the setup wizard, the branding, the operational
limits, the instance timezone, the database connection, the directory and its
synchronisation, the telemetry, the maintenance notice, the log and the restart.
Two others amount to the same in the end, and that is said rather than hidden:
whoever holds `roles:write` can tick `settings:manage` on a role, and whoever
holds `users:write` can hand that role out. None of the three is given to an
account nobody chose one by one — the directory's default role, a customer's
account.

### The built-in administrator

Every installation has the built-in administrator before anybody has chosen
anything: it is how you get in. Its role, `admin`, holds every right on the staff
side, tickets included — a ticket is the organisation's record rather than one
person's, so there is nothing in it the account that runs the installation must be
kept from, and whoever sets up queues has to be able to open a ticket to see that
what they set up works. It does not hold `portal:use`, which means something only
for an account linked to a contact.

Backups are not a thing this application does: there is no permission for them, no
endpoint and no screen. They are external, and
[`deploy/OPERATION.md`](../deploy/OPERATION.md) says what to copy for each dialect.

**The three roles the application ships with are shown, not edited** — not their
name, not their description, not their rights — and none of them can be deleted.
They are the answer to questions asked elsewhere: the role every new account gets,
the one the directory synchronisation assigns, the one a customer's account is
given, and the words the interface translates for them. A role that should grant
something different is a new role, which is a minute's work and says what it is.

**The rights are listed in words**, grouped by area, with the identifier beside
each — the identifier is what the API takes and what a directory configuration
stores, so it stays readable — and a legend under them saying what each one
actually allows.

### The interface

**Everything you press on a small screen is at least 44px tall** — the size a
finger needs, and the number both platform guidelines settled on. Measured on a
390px screen before this was set: the button that dismisses a notice was 18px,
a permission switch 22, the one that reveals a password 26, the fields 38.
Height only; nothing is enlarged to read, there is simply more of it to hit.

**The navigation folds into one control below 900px.** As a row that scrolled
sideways on a telephone it showed three points and hid the rest, with nothing on
screen to say the others were there — and because the bar was as wide as the
row, the *window* scrolled in its place, so the page could be dragged sideways.
Behind the burger they are a list, one per line, which says how many there are.
It closes on choosing a point, on Escape, and on a press anywhere else.

**Appearance belongs to the person, not to the machine.** Light or dark is
stored on the account, like the language, and applied when that account signs
in. Signed out — and on a fresh account that has never chosen — the screen
follows the time of day. It used to live in the browser, which is right for one
person with one laptop and wrong everywhere else: the next person at a shared
machine arrived to the last one's dark mode.

Everything else this interface keeps in a browser is keyed by account id and so
cannot reach anybody else: the screen each account was last on, whether the
browser's zone and language have been offered, and, for the length of one tab, a
form somebody had half filled in. The exceptions are impersonal — the instance's
own name and mark, and the appearance last applied, cached so the first paint is
not generic.

**The release feed is asked at most four times a day**, however many
administrators are looking at the screen. It used to be asked once a minute at
most, which sounds careful and is not: GitHub allows an unauthenticated caller
sixty requests an hour *per address*, every administrator's sign-in starts a
check, and every open tab repeats it hourly — so an installation with a few
administrators spent its allowance and got a 403 that reads as a permission
problem with somebody else's service. A failed lookup no longer erases the last
good answer either: the card shows the version it knows, with the trouble noted
beside it. An installation that sets `UPDATE_TOKEN` gets five thousand an hour
and never comes near either number.

## API access with personal tokens

Every user can issue their own tokens under **My account**:

```bash
curl -H "Authorization: Bearer gts_xxxxx" https://your-host/api/v1/me
# or
curl -H "X-API-Token: gts_xxxxx" https://your-host/api/v1/me
```

A token carries **no rights of its own**. Every request is evaluated against
the owner's role *at that moment*, so changing or revoking a role takes effect
on the very next call, and a token can never outrank the person it belongs to.

Only a hash of the token is stored; the value itself is shown once, at
creation, and cannot be recovered afterwards. Tokens can be given an expiry and
can be revoked at any time.

Interactive documentation is at **`/api-docs`**, and the OpenAPI specification
it renders is at `/openapi.json` — load that into Swagger UI, Postman or
Insomnia if you prefer those.

## LDAP

The administrator configures a directory under **Settings**. When it is
enabled, passwords are checked against the directory by binding as the user.

This application is a directory *client*. It does not run one and does not need
one: an installation with an Active Directory or an OpenLDAP points at what it
already has, and an installation with neither carries on with its own accounts,
passwords, passkeys and two-factor codes. For the two cases where neither is
true — trying the feature out, or genuinely wanting a directory and having none
— [`deploy/compose.ldap.yaml`](../deploy/compose.ldap.yaml) runs one beside the
application, and its header says which fields to type where.

Accounts are also created on first successful sign-in, so someone can start
working without being provisioned first. Local-only accounts keep working
alongside directory ones. Roles and permissions always stay local — the
directory decides *who you are*, this application decides *what you may do*.

An account created here under the same address — by an administrator, or by an
import — is taken over the first time its owner signs in through the directory,
with everything it already had. From then on it is a directory account: it keeps no local
password, so it is no longer asked for a new one and the initial password no
longer opens it, and a synchronisation treats it like any other. An account that
administers this installation is never taken over.

Nor is one that already belongs to another directory entry. A directory account
remembers its entry by the identifier the card names — `entryUUID`, or
`objectGUID` in Active Directory — so somebody given an address another entry
had before is not handed that entry's account and what it holds: the sign-in is
refused as vaguely as a wrong password, and the log says at WARN which entry the
account belongs to. It stays the predecessor's until a synchronisation removes
it, which happens once their entry has left the directory; the next sign-in then
starts an account of the newcomer's own. Changing the identifier attribute on the
card forgets the identifiers recorded under the old one, and each account is
matched by its address again until its owner next signs in.

### Signing in with a Kerberos ticket

Where people sign in to a Windows domain, their browser can present its Kerberos
ticket instead of a password. The operator places a keytab on the server and
names it in `KERBEROS_KEYTAB`; the ticket's owner is then looked up in the
directory configured here, and signed in to the same account the directory's
password reaches — second factor included. The sign-in screen tries a ticket by
itself and falls back to the form without one.
[`deploy/OPERATION.md`](../deploy/OPERATION.md) says how to make the keytab and
what the browsers need.

### Synchronisation

Under **Settings → Directory synchronisation** the whole directory is
reconciled with the local accounts:

- Accounts the directory has and this installation does not are **created**.
- Accounts the directory no longer holds are **deleted**, together with their
  API tokens, passkeys and sessions. The tickets they owned or were responsible
  for stay, with nobody in that place.

**The directory is only ever read.** Nothing is written back to LDAP.

**A run takes `settings:manage`**, like the card it is configured on — the
preview, the button and the schedule beside them. The schedule goes with the
buttons rather than with the fields, because it is the same deletion performed
later and unattended: a field open to more people than the button above it would
let the caution the button was given be walked around by typing five numbers.

Set the schedule under *Settings*, or `LDAP_SYNC_SCHEDULE` for a starting value
in the environment; it is empty by default because a run deletes accounts
irreversibly, and an automatic one deletes them with nobody looking. It runs on
the server's clock - UTC in the shipped container, which sets no `TZ` - and not
in the instance timezone, so `0 4 * * *` is four in the morning in Greenwich. Use
**Preview** first, which reports exactly which accounts would go and how many
unfinished tickets each one would leave without an owner. The real run asks for confirmation
naming those numbers.

Four guards make a broken directory answer non-destructive:

| Guard | Reason |
| --- | --- |
| A failed directory read aborts with an error | An outage must never be read as "everybody left" |
| An empty result deletes nothing | Almost always a wrong base DN or filter, not a mass departure |
| `LDAP_SYNC_MAX_DELETE_RATIO` (default 0.5) | A truncated or misfiltered answer would otherwise look like half the company leaving |
| Local and built-in accounts are never touched | They were never in the directory, so its silence says nothing about them |

Listing is paged, because directories commonly cap a plain search at 500 or
1000 entries — a silent truncation would read as "everyone beyond the first
page has left".

Every run that removes or adds an account is listed under **Recent runs** on
the card: when it ran, whether somebody confirmed it against a preview, and how
many accounts it took or brought, and how many tickets were left without an
owner. It names nobody — the people
a run removed are the people the deletion erased — and the log, which does name
them, can be switched off by its level; this list cannot.

## Signing in through your organisation's provider

Under **Settings**, *Sign-in provider (OpenID Connect)* points the installation
at a provider - Keycloak, Authentik, Entra ID, Google - by its issuer address,
the client ID and secret it registered this installation as, and the name the
sign-in form calls it by. The card shows the address to register at the
provider for sending people back: this installation, followed by
`/sign-in/openid`. *Test the provider* reads its discovery document before
anybody depends on it.

**Nobody is signed in by an address.** An identity at the provider opens an
account only once that account's owner has linked it, under **My account**,
while signed in - with their password and its second factor. No account is
matched on the address a provider reports, and none is ever made for somebody
the provider knows: an installation whose accounts its administrator makes
should not have them made by whoever can sign in at Google. From then on the
provider's sign-in is theirs to use instead of the password, with the
provider's own second factor; the password keeps working beside it, and
*Unlink* takes the link away.

What holds the round trip together:

- **The browser that left is the one that comes back.** The round's state is
  kept in a cookie only this installation can set, so a code the provider sent
  one browser back with cannot sign another one in - which would otherwise let
  somebody have another person's browser signed in as them.
- **Only what the provider signed is believed.** The code is traded with the PKCE
  verifier the round was begun with, and the identity token counts only when it
  carries the provider's published signature, this installation's client, a
  time that has not passed and the round's own nonce.
- **The built-in administrator and directory accounts stay on their passwords**,
  for the reasons a passkey is refused them: the first is the guaranteed way
  back in, and the second has to go through the directory, which is where an
  organisation ends somebody's access.
- **A provider is reached over HTTPS**, unless it runs on this machine: over plain
  HTTP anybody on the way could answer for it and sign anybody in as anybody.
  The client secret is sealed with `SECRET_KEY` like the directory's bind
  password.

## The administration log

Under **Settings**, *Changes to the installation* lists who changed how the
installation is run, newest first: an account created, corrected, given another
role, given a new password or deleted; a role created, changed or deleted;
accounts or roles imported; each settings card saved; maintenance mode
switched on or off; the directory synchronised; the application restarted, updated or set up. Everybody holding `settings:manage` can read it, and nobody else.

A change is recorded once it has gone through - a refused request changed
nothing and is not there, and neither is *Check the file* before an import,
which is the import's own request told not to write - by a middleware working from one table of routes, so
no screen and no API client can change accounts, roles or settings past it.

It keeps ids and reads the names when it is shown. An account renamed since
appears under its new name; an account erased since appears by its number alone,
because the log outlives the people it named without keeping their names. That is
also why erasing an account leaves it alone, as it leaves what somebody wrote in
a ticket: the record of what an administrator did to everybody else's accounts is
not theirs to take with them. Unlike the live log beside it, it survives a
restart.
