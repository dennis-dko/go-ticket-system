<h1 align="center"><img src="internal/interface/web/asset/favicon.svg" alt="" width="96"><br>Go-Ticket-System</h1>

<p align="center">
  <b>Project time tracking in a single self-contained binary.</b><br>
  A web interface, a REST API and an Android app for recording where working
  hours go — self-hosted, on SQLite, PostgreSQL or MySQL.
</p>

<p align="center">
  <a href="https://github.com/dennis-dko/go-ticket-system/actions/workflows/ci.yml"><img src="https://github.com/dennis-dko/go-ticket-system/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/dennis-dko/go-ticket-system/releases/latest"><img src="https://img.shields.io/github/v/release/dennis-dko/go-ticket-system?label=release" alt="Latest release"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/dennis-dko/go-ticket-system" alt="Go version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/dennis-dko/go-ticket-system" alt="MIT licence"></a>
</p>

<p align="center">
  <a href="#get-started">Get started</a> ·
  <a href="#a-tour">A tour</a> ·
  <a href="#on-a-phone">On a phone</a> ·
  <a href="#how-it-fits-together">How it fits together</a> ·
  <a href="#what-it-does">What it does</a> ·
  <a href="#documentation">Documentation</a>
</p>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="doc/image/calendar-dark.png">
  <img src="doc/image/calendar.png" alt="The calendar: a month of booked hours, with two days of leave, a day off sick and a public holiday marked">
</picture>

## Why Go-Ticket-System

- 📦 **One file to run.** The interface, the API, the schema migrations and the
  timezone database are compiled into one binary — for Linux, Windows and macOS,
  and as an image for amd64 and arm64. Nothing to unpack beside it.
- 🔒 **Your hours stay yours.** Every project belongs to one person, and nobody
  else — the administrator included — can read, total or correct anybody's time.
  An installation may switch on managers for the people an administrator assigns
  them, and then those people are told who, and see their name on every entry a
  manager touched. No team totals, no approval queue.
- 🗄️ **The database you already run.** SQLite in a single file, or PostgreSQL or
  MySQL. Started without one, the binary serves an installer on its own port and
  waits.
- 🔑 **Signing in the way your organisation does.** Passwords with an
  authenticator app, passkeys, LDAP and Active Directory with Kerberos single
  sign-on, and OpenID Connect providers such as Keycloak, Authentik, Entra ID or
  Google.
- 📈 **Built to be operated.** Health endpoints, Prometheus metrics,
  OpenTelemetry traces, a live log, a maintenance mode, updates from the settings
  screen and HTTPS with Let's Encrypt.
- 🌍 **For the people using it.** English and German, light and dark, from a
  telephone to a wide screen — and installable as an app.

## A tour

Every picture below is taken from the running application by
[a test](test/browser/readmepicture_test.go) that books a year of somebody's
working time first — `task readme:pictures` takes them again.

<table>
  <tr>
    <td width="50%" valign="top">
      <img src="doc/image/week.png" alt="The week grid: projects as rows and days as columns, with the hours booked, what each day asked and the balance underneath">
      <p><b>The week as a grid.</b> Type hours straight into the cells; what each day asked and the balance sit underneath, and <i>Copy last week</i> fills the days that are still empty.</p>
    </td>
    <td width="50%" valign="top">
      <img src="doc/image/stopwatch.png" alt="The stopwatch, running for an hour and twenty minutes on the website relaunch">
      <p><b>A stopwatch that counts seconds.</b> Switch to the next project without losing one; a start pressed late can be corrected, and a clock left running is booked up to when the work ended.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/activity.png" alt="Charts of the month's hours per project and per activity">
      <p><b>Your own figures, as charts.</b> Hours per project, and per activity: write <code>#meeting</code> or <code>#review</code> into a description and it is counted — nothing to set up first.</p>
    </td>
    <td valign="top">
      <img src="doc/image/report.png" alt="A report of the month as a pie chart, one slice per project">
      <p><b>Reports in the shape the question needs.</b> Bars, columns or a pie, each project keeping its colour in all three, and a PDF of the same.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/year.png" alt="The year as tiles, one per day, darker for more hours, with public holidays and days away outlined">
      <p><b>The year at a glance.</b> A tile a day, as dark as the hours on it; public holidays and days away outlined rather than told by colour alone, beside the year before.</p>
    </td>
    <td valign="top">
      <img src="doc/image/project.png" alt="The project list: one project in the last fifth of its hour budget, one past it, and the hourly rate of those that are charged for">
      <p><b>Projects with budgets and rates.</b> The list says how much of a budget is booked, marked from four fifths and in red once it is past, and what a project is charged an hour.</p>
    </td>
  </tr>
  <tr>
    <td colspan="2" valign="top">
      <img src="doc/image/statement.png" alt="A statement of work for the month: the hours of three billable projects, their hourly rates and what they come to, totalled in euros">
      <p><b>A statement of work at the end of the month.</b> The hours of the projects you charge for, at their rate, totalled per currency, as a PDF or a workbook. Not an invoice: the number, the tax and the customer stay yours to write.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/overtime.png" alt="The overtime balance of the month, day by day">
      <p><b>An overtime balance against your own target.</b> Choose a day, and from then on every working day counts — less what a public holiday or a day away takes off it.</p>
    </td>
    <td valign="top">
      <img src="doc/image/workday.png" alt="The working time of the month: when each day began and ended, its breaks, and the hours booked beside them">
      <p><b>Your working time, on record.</b> When each day began and ended and how long the breaks were, beside the hours booked; a gap shows as one, and the month leaves as a PDF.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/entry.png" alt="The entry list narrowed to the month's meetings">
      <p><b>Every entry, findable.</b> Narrow the list by project, period or words in the description — the export follows the same filter — and every correction keeps the version it replaced.</p>
    </td>
    <td valign="top">
      <img src="doc/image/signin.png" alt="The sign-in form, offering a password, a passkey and the organisation's provider">
      <p><b>Sign in your way.</b> A password and an authenticator app, a passkey, your organisation's provider — or, inside a domain, the Kerberos ticket the browser already holds.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/log.png" alt="The live log under Settings, filtered by level">
      <p><b>Operated from the browser.</b> The process log, live; the database, the directory, telemetry, maintenance, updates and a restart, all under <i>Settings</i>.</p>
    </td>
    <td valign="top">
      <img src="doc/image/installer.png" alt="The installer asking for the setup token and a database">
      <p><b>Set up in the browser.</b> Without a database the binary serves this screen, guarded by a token from its log, and then takes its port over in the same process.</p>
    </td>
  </tr>
  <tr>
    <td valign="top">
      <img src="doc/image/calendar-de.png" alt="The calendar in German">
      <p><b>English and German</b>, every screen and every refusal — dates and figures written the way the reader's language writes them.</p>
    </td>
    <td valign="top" align="center">
      <img src="doc/image/phone.png" alt="The stopwatch on a telephone" width="240">
      <p><b>From a telephone to a wide screen</b>, and installable as an app with the installation's own name and logo - which opens without a network, and keeps what is booked meanwhile until it is back.</p>
    </td>
  </tr>
</table>

## How it fits together

One process answers everybody: the browser, the phone, the scripts, the
calendar apps.

```mermaid
flowchart TB
    people(["People<br/>browser · telephone"])
    scripts(["Scripts and tools<br/>personal API token"])
    calendars(["Calendar apps<br/>iCalendar subscription"])

    subgraph binary["go-ticket-system-web — one binary"]
        installer["Installer<br/>until a database exists"]
        ui["Web interface<br/>embedded"]
        api["REST API<br/>/api/v1 · /api-docs"]
    end

    db[("SQLite · PostgreSQL · MySQL")]
    directory["LDAP · Active Directory<br/>Kerberos"]
    provider["OpenID Connect provider"]
    telemetry["Prometheus · OpenTelemetry"]

    people --> ui --> api
    scripts --> api
    calendars --> api
    installer -. chooses .-> db
    api --> db
    api -. sign-in .-> directory
    api -. sign-in .-> provider
    api -. metrics · traces .-> telemetry
```

Everything a person books ends up in one place, and every figure is read from
there:

```mermaid
flowchart LR
    stopwatch["Stopwatch"] --> entry
    form["Entry form"] --> entry
    grid["Week grid"] --> entry
    workbook["Spreadsheet import"] --> entry
    entry[("Time entry<br/>day · hours · project · activity")]
    entry --> calendar["Calendar and year"]
    entry --> figures["Charts and reports<br/>PDF · spreadsheet"]
    entry --> balance["Overtime balance"]
    entry --> feed["Calendar subscription"]
    plan["Planned week<br/>public holidays · days away"] --> balance
```

The first start, when nothing is configured yet:

```mermaid
sequenceDiagram
    actor You
    participant App as go-ticket-system
    participant DB as Database
    You->>App: start it, with no database configured
    App-->>You: installer on :8000, setup token in the log
    You->>App: the token, and SQLite, PostgreSQL or MySQL
    App->>DB: test the connection, create the schema
    App-->>You: same port, same process — signed in, choose a password
```

## Get started

**With Docker Compose** — the application and a PostgreSQL beside it, from the
[`deploy/`](deploy/) folder:

```bash
cp .env.example .env    # DB_PASSWORD is required
docker compose up -d
```

**As a single binary** — download `go-ticket-system-web` for your platform
from the [latest release](https://github.com/dennis-dko/go-ticket-system/releases/latest)
and start it. It is the whole installation.

```bash
chmod +x go-ticket-system-web_*_linux_amd64
./go-ticket-system-web_*_linux_amd64
```

Then open <http://localhost:8000>. Without a database the binary serves the
installer, which asks for the setup token printed in its log; with one, sign in
as `admin@local` with the initial password `changeme123`, and the first thing
asked for is a new one. Windows warns about an unknown publisher the first time
— [here is why](doc/getting-started.md#windows-says-unknown-publisher).

**From the source**, with [Task](https://taskfile.dev):

```bash
task dev DB=sqlite      # or: go run ./cmd
```

Running it for real — behind a proxy, with HTTPS, as a service — is
[`deploy/OPERATION.md`](deploy/OPERATION.md).

## On a phone

The Android app is the installation's own web interface, in an app: every view,
card and form the browser has, in the same design, saved through the same API
into the same database - and anything the web interface learns in a release,
the app has with it. It is connected by the code a new token is shown with under
**My account → API tokens**: the app scans it with its own camera screen, or the
phone's camera opens it, and either way the app shows the installation it is
asked to connect to and waits for a tap, because a code is a link any page could
open. The address and the token can be typed in instead.

<table>
  <tr>
    <td width="25%" valign="top">
      <img src="doc/image/app-connect.png" alt="The app's first screen, drawn as the web interface's sign-in card: a button to scan a token's code, and the address and the token to type in instead">
      <p><b>Connected once.</b> By the code a token is shown with, or by its address and value typed in.</p>
    </td>
    <td width="25%" valign="top">
      <img src="doc/image/app-scan.png" alt="The app's camera screen: the camera's picture under a veil, and a framed window to hold over a token's code">
      <p><b>Scanned in the app.</b> Its own camera screen fills the form; it connects only once Connect is pressed.</p>
    </td>
    <td width="25%" valign="top">
      <img src="doc/image/app-stopwatch.png" alt="The web interface in the app: the stopwatch running on a project for an hour and twenty minutes, and the form that books time below it">
      <p><b>The web interface.</b> Every screen the browser has, in the same design.</p>
    </td>
    <td width="25%" valign="top">
      <img src="doc/image/app-menu.png" alt="The web interface's menu opened in the app over the page: the appearance, the language, signing out and the tabs">
      <p><b>Its menu.</b> The tabs, the appearance, the language and the way out.</p>
    </td>
  </tr>
</table>

These are taken from the app on an emulator by [a script](test/camera/android.sh)
— `task readme:pictures:app` takes them again.

The token signs the app in, so no password and no second factor are typed on
the phone, and it keeps the web interface signed in for as long as the token is
good. Signing out in it disconnects the app as well, so a phone handed on does
not sign its next holder back in; and a phone that is lost is signed out by
revoking its token under **My account → API tokens**, at once - from wherever it
is revoked, the app then stops its reminders and asks for a new token's code. A file the web interface saves - a report, an
export - is saved where the phone's file picker says, and a file it asks for
is chosen there too. The reminders are notifications of the app's own, whether
it is open or not, once they are allowed under **My account → Reminders**: the
stopwatch past the hours chosen, and the last working day with nothing booked.
Two things stay in a browser: passkeys and signing in through your organisation,
which are set up there.

**It belongs to the release it was built with.** Download
`go-ticket-system-mobile_<version>.apk` from the
[release](https://github.com/dennis-dko/go-ticket-system/releases) the
installation runs - **My account → Install the Android app** shows it as a code
the phone's camera downloads it from - and open it on the phone; Android asks once whether the
browser or file manager it was opened from may install apps. Every release is
signed with the same key, so the next one installs over it. An installation
whose certificate comes from an authority of its own needs that authority
installed on the phone, as the phone's browser does.

## What it does

| Area | What you get | Read more |
| --- | --- | --- |
| **Recording** | Stopwatch, entry form and week grid; templates for what is booked often; changes to many entries at once; spreadsheet import and export | [Features](doc/feature.md) |
| **History** | Every correction keeps the version it replaced, for the entry's owner | [Features](doc/feature.md#business-rules) |
| **Evaluating** | Charts per day, project and activity, reports as bars, columns or a pie, the year as tiles, statements of work, PDF exports | [Features](doc/feature.md) |
| **Projects** | Private to their owner, with a period, a status, an hour budget and an hourly rate | [Projects](doc/feature.md#projects) |
| **Targets and time off** | A planned week, the public holidays of seventeen countries and their states or regions, leave with its entitlement and carry-over, sick days and days taken against overtime | [Overtime](doc/feature.md#overtime) |
| **Working time** | When each day began and ended and its breaks, for each person's own record | [Working time](doc/feature.md#your-own-record-of-your-working-time) |
| **Calendar subscription** | Booked days and days away in Outlook, Google or a telephone's calendar | [Calendar](doc/feature.md#your-days-in-your-own-calendar) |
| **Accounts and roles** | Roles and their rights administered at run time; an administrator who runs the installation without keeping hours in it; managers for the accounts assigned to them, where an installation switches them on | [Access](doc/access.md) |
| **Sign-in** | Passwords with TOTP, passkeys, LDAP and Active Directory with synchronisation, Kerberos, OpenID Connect | [Sign-in](doc/access.md#ldap) |
| **API** | REST under `/api/v1`, described at `/api-docs`, reached with personal tokens that never outrank their owner - one of which signs the Android app in | [API](doc/api.md) |
| **Operations** | Health, metrics, traces, live log, maintenance mode, updates, restart, HTTPS with Let's Encrypt, the installation's own name and logo | [Configuration](doc/configuration.md) |

## Documentation

| Page | What it covers |
| --- | --- |
| [Getting started](doc/getting-started.md) | The installer, the first sign-in, the setup wizard and the tour |
| [Features](doc/feature.md) | Everything the application does, and the rules it keeps while doing it |
| [Accounts, roles and sign-in](doc/access.md) | Rights, the administrator, API tokens, LDAP, Kerberos and OpenID Connect |
| [Security](doc/security.md) | Sessions, CSRF, passkeys, two factors, HTTPS, and what a copy of the database gives away |
| [Configuration](doc/configuration.md) | Where each setting comes from, and what the interface can change |
| [API](doc/api.md) | Connecting an app, what a refusal looks like, and what the metrics carry |
| [Architecture](doc/architecture.md) | The four layers, and the decisions that would otherwise surprise |
| [Development](doc/development.md) | The environments, and the checks a change has to pass |
| [Releases and deployment](doc/deployment.md) | How a release is made and what it carries |
| [Operations manual](deploy/OPERATION.md) | For whoever runs it: the four ways to deploy, backups, restarts, health |

Built on [GoFr](https://gofr.dev), structured after [gogs](https://github.com/gogs/gogs).

## License

[MIT](LICENSE).
