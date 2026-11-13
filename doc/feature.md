# Features

What the application does, and the rules it keeps while doing it.

The ticket system itself - queues, tickets, contacts, the knowledge base and the
customer portal - is being built. What follows is what an installation already
has, and what every part of the ticket system will stand on.

## What is included

| Area | Implementation |
| --- | --- |
| Storage | SQLite (pure Go, no cgo), PostgreSQL or MySQL - chosen in the installer, never defaulted |
| Concurrency | SQLite runs in write-ahead logging, so a save and a page load do not refuse each other |
| Schema | GoFr migrations, applied automatically at start-up |
| API | REST under `/api/v1`, documented at `/api-docs` |
| Web interface | Embedded via `go:embed`, vanilla JS with no build step |
| Access control | RBAC with roles administered at run time; bcrypt password hashes |
| Sign-in | Session cookies, optional passkeys and TOTP two-factor per user, a browser's Kerberos ticket where a keytab and a directory are configured, and an OpenID Connect provider - Keycloak, Authentik, Entra ID, Google - for the accounts their owners linked to it |
| Live log | The process log, filterable and searchable, for whoever administers |
| Administration log | Who created, changed or deleted an account or a role, saved a setting, switched maintenance mode or restarted the application, kept across restarts and readable under *Settings* by whoever administers |
| Version | The running build in the footer of every page |
| API access | Personal tokens, scoped by the owner's current role |
| Android app | The installation's own web interface in an app, so everything the browser does in the same design, connected by scanning the code a new token is shown with, which signs it in; released with every version as `go-ticket-system-mobile_<version>.apk`, signed with one key so each release installs over the last |
| Notifications | Switched on under *Settings*, off as shipped: each person then lets a browser show what happens to their tickets as notifications, and has them pushed to it while no page is open, under *My account* |
| Public holidays | The public holidays of seventeen countries - Austria, Belgium, Czechia, Denmark, Finland, France, Germany, Ireland, Italy, Luxembourg, the Netherlands, Norway, Poland, Portugal, Sweden, the United Kingdom and the United States - worked out in the binary rather than fetched from anywhere. A country is chosen first, Germany standing ready, and a second picker follows where its holidays differ: a German state or a British nation always, and Alsace, Moselle, South Tyrol, the Azores or Madeira where they keep a day or two more than the rest of their country. The installation chooses under *Settings*. A holiday a country moves off a weekend is kept on the weekday it moves to, and said to be. Nothing reads them yet: they are kept for the service hours a service level is measured in |
| Everything about me | Everything the installation keeps about your account, as a ZIP with a JSON file per part - exactly what erasing it would delete or release, without the hashes, seeds and keys - from *My account*, for anybody signed in |
| Transport | Optional HTTPS with automatic Let's Encrypt certificates |
| Installable | A manifest and icons per installation - its own name and logo - so the page installs as an app on a telephone or a desktop, with a shortcut to *My account*. Browsers offer this over HTTPS or on localhost only. *My account* shows the installation's address beside the offer, as text and as a QR code, to open it on a telephone by scanning it rather than typing it |
| Offline | The installed application opens without a network from the shell it kept, and says the installation cannot be reached rather than showing a dead page. Nothing the installation answers is ever shown from a copy. Over HTTPS or on localhost only, like installing |
| Operations | Health and liveness endpoints, Prometheus metrics, tracing (all from GoFr), administered under *Settings* |

## Timezones

An administrator sets one **instance-wide** zone under *Settings*; an
individual can override it under *My account*. An empty personal setting means
"follow the instance", which is the normal case — that way changing the
instance zone moves everyone who has not deliberately opted out.

This is not a display detail. The zone decides which **calendar day** a moment
falls on, which is the day a ticket's number is taken from and the day a service
level's hours are counted in. Zones are validated when saved, because a
plausible but wrong name like `Europe/Munich` would otherwise be stored and then
fall back to UTC at every use, with nothing on screen to show it.

The tz database is compiled into the binary (`time/tzdata`), so zones resolve
identically on a scratch or distroless container with no zoneinfo files of its
own.

## Everything about you

Under **My account**, *Everything about me* downloads what this installation keeps
about your account as a ZIP: a JSON file per part - the account, the tickets you
own or are responsible for, the contact you sign in as, the tickets you watch, your
notifications and the queues you are told about, the articles you wrote, and the
personal tokens, passkeys, sessions, push subscriptions and provider links you
have - each an array of rows with the columns as they are stored, and
`about.json` saying what the archive is. It is the right of access and of
portability (GDPR articles 15 and 20), open to anybody signed in whatever their
role lets them do here, and only ever about their own account.

It holds exactly what erasing the account would delete or release, read from the
same list of tables, so nothing can be erased that could not have been handed
over first. An article is handed over as where and when it was written rather
than as its text: an answer quotes the customer it answers, and the export of one
person is not the place for another's words. What it leaves out is what would be
a way in for whoever finds the file: the password's hash, the second factor's
seed, the hashes of tokens and sessions, and a passkey's key material. About
those it says that they exist, when they were made and when they were last used.

## Spreadsheets

The accounts and the roles go out and come back as a real **.xlsx** workbook,
from the tab they belong to: accounts with their role and whether the password
lives in the directory; roles with a column per permission holding yes or no.
Names rather than identifiers, because a column of ids is not something anybody
can fill in by hand.

A column per right rather than one cell listing them, which is what makes the
roles sheet honest. A list in a cell reads well and imports badly: a typo in
`tickets:read, tickets:wrote` is a right silently dropped, and nothing about
it looks wrong until somebody cannot open a screen. A column is a question with
two answers, and a heading naming a right this application does not enforce is
refused by that name — which also catches a file exported from a different
installation. A file may leave rights out; a column somebody deleted is simply
a right the import does not touch.

Two tables deliberately have no sheet. A token's secret exists once, at the
moment it is created, and is not in the database to export. A passkey is bound
to the device holding it and means nothing anywhere else.

The column headings are written in the language the export was asked for, and a file
exported in any of them imports again: the heading row is skipped by position, and
translated values are recognised in every language.

Not comma-separated text. A CSV is what Excel mangles: the separator depends on the
machine's locale, dates are re-interpreted on opening, and a cell containing a
semicolon quietly becomes two columns — none of it recoverable once somebody has
saved over the file.

The import is **two steps, and all or nothing**:

1. *Check the file* reads it and shows every row — which would be written, which
   would not, and why, named by its line in the sheet so you know where to look. It
   writes nothing.
2. *Import* is only offered when every row can be written, and runs in one
   transaction.

A file half-imported leaves nobody able to say which half, or which rows came
from it, which is why a single refused row refuses the file. Every row goes
through the rules the API enforces — by calling them, not by restating them.

Accounts are matched on the mail address and are **changed**, not created: a new
account needs a password, and a password that arrives in a spreadsheet is a password
that gets mailed around. Roles are matched on the name and are both — everything a
role is fits in a row, so a file can describe one that does not exist yet without
carrying anything that has no business being in a spreadsheet. The roles the
application ships with are the exception the role editor already makes: their
rights cannot be changed from a cell any more than from the screen.

Why a row was refused is written in the reader's language, like the headings above it
and the values in it. The reasons are a fixed set, so each one travels as a code with
whatever its sentence names — the row number, the offending word — rather than as a
sentence assembled on the server in English.

## The logo, and what a tab does with it

A logo is a **PNG or a JPEG**, up to 256 KB. Not an SVG: the same file that
renders perfectly in the header and on the sign-in screen can be refused as a tab
icon, silently, by an engine with its own rules about what it will rasterise —
nothing in the response says so, and the tab is simply empty.

**It is converted when it is saved**, into the size each place draws it:

| | Kept at | Drawn at |
| --- | --- | --- |
| Header | 440×80 | 220×40 |
| Sign-in card | 656×192 | 328×96 |
| Browser tab | 64×64, square | 16, 32 or more |

Twice the drawn size, because a screen at twice the density draws twice as many
pixels for the same box. Derived once and stored, so a page load reads them
rather than making them, and a restart has nothing to redo.

That matters most for the tab. What an installation uploads is a wordmark made
for a header — a few thousand pixels across, often twice as wide as it is tall —
and handed to a browser at that size, every decision is the browser's: whether to
accept it at all, how to fit two-to-one into a square, whether to bother. The
answers differ by engine, and one that decides against shows nothing and says
nothing. The tab's version is padded out to a square for the same reason; the
other two are not, because a header places the mark itself.

It also shrinks what every visitor downloads. The sign-in screen used to receive
the whole original inside the branding response, before anybody had signed in.

**Which part of the logo each place uses is yours to choose.** Press a preview on
the appearance screen and drag: any part, in any shape, from any corner. The
selection opens on the shape of the place it is for, because that is the one
selection nothing has to be done to, but nothing holds it there — and a part that
is not that shape is fitted into the place rather than stretched to fill it. A
wide header can take the whole wordmark; a browser tab has sixteen pixels, and the part worth
keeping there is usually the mark at one end rather than whatever falls in the
middle. Nothing has to be chosen — a logo starts as all of itself in all three
places, which is what most installations will leave it as.

The choice is remembered per place and per logo. Uploading a different logo
forgets it, because a selection made against one picture means nothing against
another.

Fitted, not cropped: nothing of the logo is cut. That does mean a wide wordmark
becomes a thin strip at the sixteen pixels a tab draws, which is a fact about
wordmarks rather than about this. The appearance screen shows the logo at exactly
that size beside the header and sign-in previews, so it is seen before saving. A
square mark is what reads best there — many companies keep one for exactly this.

**Until a logo is configured, the header and the sign-in card carry nothing.**
Those two slots belong to whoever runs the installation, and filling them with
our mark makes an unbranded installation look branded by somebody else. The
application's own mark — an hourglass — has its own places: the browser tab, the
`.exe`, and the button beside the title that leads to the welcome screen. It is
drawn (`internal/interface/web/asset/favicon.svg`) rather than a font character,
and `go run ./build/icon > build/icon.ico` redraws the Windows copy from the same
geometry whenever it changes.

**The tab can be named separately from the header**, under *Browser tab* beside
the title. The two are one name until the header's is too long to be a tab: a
browser cuts a tab off after a couple of dozen characters, and somebody has six
of them open — "Ticketsystem der Beispiel GmbH & Co. KG" reads as
"Ticketsystem der B…" in every one. Left empty, the tab keeps the title, which
is what every installation configured before this existed goes on doing. It
translates like the other texts.

**A language you have written nothing for follows the base**, the same way an
unconfigured logo falls back to the shipped mark. The form fills each language
with the base text so you can see what a reader of it currently gets; leaving
that untouched means exactly what it looks like — nothing written here. Rename
the installation later and every untranslated language follows along.

Saving a mark reloads the page, because no engine takes a new tab icon from a
link swapped in afterwards — and the reload comes back to where you were on the
screen rather than to the top of it.

## Business rules

The server enforces these; the interface merely also hides what is not allowed:

- Email addresses are unique.
- The built-in administrator cannot be deleted, and keeps a role that can still
  administer.
- No role the application ships with can be deleted, renamed or changed — not its
  rights, in either direction, and not its description. The `admin` role is the
  system role on top of that, the one that always administers.
- A role has to grant at least one permission. One that grants nothing can be assigned,
  and whoever holds it signs in to an interface with nothing on it, which reads as a
  broken installation rather than as a decision.
- A role still assigned to someone cannot be deleted.
- The role the directory hands to every account it brings in cannot be one that
  administers the installation.
- Deleting an account that owns unfinished tickets has to be confirmed, and the
  refusal says how many. The tickets stay, without an owner.
- An account the directory keeps cannot be renamed, readdressed or deleted here; the
  directory decides who it is, and only its role is this installation's to choose.
