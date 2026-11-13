# API

Responses are wrapped by the framework: `{"data": ...}` or `{"error": ...}`.
The full reference is at `/api-docs`; the highlights:

## Connecting an app

An app that works through this API needs two values: the installation's
address and a personal token. A token is made under *My account*, API tokens,
and the card shows both once: as text, the token's value and the address, and
together, as a QR code. The code holds a connection address:

```text
go-ticket-system://connect?token=gts_...&url=https%3A%2F%2Ftickets.example.com
```

`url` is the address the browser reached the installation at when the token
was made, scheme and host without a path, and `token` is the token itself.
Both are query-encoded, and the order of the two is not to be relied on. An
app registers the scheme `go-ticket-system`, so that scanning the code with
the phone's camera opens it, and from then on asks the API with
`Authorization: Bearer <token>`.

The Android app in this repository, released as
`go-ticket-system-mobile_<version>.apk`, is such an app. It scans the code with
a camera screen of its own as well, and fills its connect form from the code
however it was read, and connects only once whoever holds the phone has seen the
address and agreed, because a code is a link any page could open - and an app
that connected on its own would hand somebody's token to wherever such a link
pointed.

The answer that creates a token carries the same thing for a program that makes
one itself: `connectUri`, the address above, and `qr`, the code as an SVG data
URI. Like the token's value, neither is ever shown again.

The code is the credential. Whoever photographs it acts as the account until
the token expires or is revoked, so it is shown where nobody else is looking,
and a token whose code somebody else may have seen is revoked rather than kept.

An app depends on the installation's version: what it asks for and how it reads
the answers are one release's API.
`GET /api/v1/branding` answers anybody, signed in or not, and its `version`
says which release that is.

## What a refusal looks like

Every refusal names itself. The name is what something other than an English
reader can act on — the interface looks up a sentence in the reader's own
language, and a support conversation has a stable word for a thing rather than a
paraphrase of a message that has since been reworded.

```json
{
  "error": {
    "code": "probeFailed",
    "message": "the connection could not be established",
    "detail": "cannot reach the database: dial tcp 10.0.0.4:5432: connect: connection refused",
    "ref": "A7F3C2"
  }
}
```

| Field | Always? | What it is |
| --- | --- | --- |
| `code` | yes | Why, as a stable name. Never changes meaning. |
| `message` | yes | The same thing in English, for a client with no dictionary. |
| `values` | sometimes | What the sentence interpolated, kept apart so a translation can put them in its own word order. |
| `param` | field rejections | Which fields, by name, so a form can label them the way it labels everything else. |
| `detail` | failures from underneath | The original wording of whatever actually failed — a driver, a directory, a file system. Untranslatable by nature, so it is carried rather than shown. |
| `ref` | internal failures | The same string as in the log line. This is how a screenshot and a log entry are matched up. |

Two kinds of `code`. Specific ones are declared where the rule is enforced
(`deletionNeedsConfirming`, `systemRoleUndeletable`). The generic ones — the refusals that
belong to no single rule — are declared once in
[`apperror/code.go`](../internal/support/apperror/code.go): `internal`, `probeFailed`,
`unauthenticated`, `notFound`, `invalidFields`, `rateLimited`, `csrfRejected`,
`maintenance`.

Words rather than numbers, deliberately. A numbered scheme is right where the
code is all you get — a return value in a register with a table somewhere else.
These travel in a JSON body with room beside them, so the property worth having
is that the code says what it means where it is read. What is borrowed from the
numbered systems is the part that makes them work: the set is closed and nothing
may emit a reason outside it, which
`TestEveryErrorTheAPICanGiveIsNamed` checks by provoking each one against a
running instance.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login`, `/auth/logout` | Sign in and out |
| `POST` | `/api/v1/auth/token` | Exchange a personal token for a session |
| `GET` | `/api/v1/me` | Own identity and permissions |
| `PUT` | `/api/v1/me/password`, `/me/language`, `/me/timezone`, `/me/theme` | Own password, language, zone and appearance |
| `POST/PUT/DELETE` | `/api/v1/me/totp` | Two-factor enrolment |
| `GET/POST` | `/api/v1/me/tokens` | Personal API tokens |
| `GET` | `/api/v1/me/export` | Everything held about the own account |
| `GET/POST/PUT/DELETE` | `/api/v1/users`, `/users/{id}` | User administration; deleting an account that owns unfinished tickets needs `?confirm=true` |
| `PUT` | `/api/v1/users/{id}/role` | Role |
| `GET/POST/PUT/DELETE` | `/api/v1/roles`, `/roles/{id}` | Roles |
| `GET` | `/api/v1/permissions` | Every enforced permission |
| `GET/PUT` | `/api/v1/settings/...` | Branding, database, LDAP, sign-in provider, public holidays, metrics and tracing |

## What the metrics endpoint carries

GoFr measures the machinery on its own — a histogram per HTTP request
(`app_http_response`), one per SQL query (`app_sql_stats`), a goroutine gauge —
and it does so without a line of instrumentation in this repository. Spans are
the same: every request is traced by the framework's middleware, which is why
tracing works here with no span code anywhere.

What it cannot know is whether the application is doing its job. A deployment can
serve every request in milliseconds while nobody has been able to sign in since
the directory changed. So these are recorded here as well, each because
somebody would act on it:

| Metric | Says |
| --- | --- |
| `gts_signin_failures_total` | refused sign-ins, by reason — `credentials` is somebody guessing, `totp` a second factor that did not take, `directory` a directory that stopped answering, `database` this installation's own database that did |
| `gts_directory_accounts_total` | accounts the synchronisation created or deleted — the one operation that removes people |
| `gts_directory_scheduled_runs_total` | what each scheduled synchronisation came to — `completed`, `refused` by a guard, or `failed`; GoFr's own `app_cron_job_success` counts all three as a success |

None of them carries a user, an address or a ticket as a label. A label is
a time series: one per person is both a memory leak in the collector and a list
of who works here, published on a port that asks for no password.

**Before writing an alert:** a metric is published only once it has a value, so
an installation that has had no refused sign-in publishes no
`gts_signin_failures_total` at all rather than publishing it as zero. Treat an
absent series as absent — `absent()` — rather than as a healthy zero.

Operations: `/.well-known/health` and `/.well-known/alive` on the application's
own port, which is what the image's healthcheck asks — and `/metrics` on port
2121, a port of its own, outside the middleware chain, which therefore asks for
no sign-in, is not covered by TLS, and serves Go's profiling endpoints under
`/debug/pprof/` beside the metrics. Reach that one from your monitoring, not from
the internet, or switch it off under *Settings*.

This sentence used to put all three on 2121. They are not: the two health paths
answer on the application port and 404 on the metrics port, which is why
`wget http://127.0.0.1:8000/.well-known/alive` is what
[`Dockerfile`](../Dockerfile) and [`deploy/compose.yaml`](../deploy/compose.yaml) both
run.

Neither of them means as much as it looks like. `/.well-known/health` answers
`200` even when the SQL datasource is down — the state is in the body, as
`"status": "UP"` or `"DEGRADED"` — so alerting on the status code alerts on
nothing. And `/.well-known/alive` answers `200` while the **installer** is
running, because the installer serves its page for every path, so a container
sitting on the installer with no application behind it reports healthy. The
honest liveness check for "is the application actually up" is
`/api/v1/branding` carrying a `version`, which is what the release workflow's
smoke test asks.

`METRICS_PORT` switches the endpoint off only on exactly `0`. Any other value it
cannot parse — `off`, `false`, `-1`, empty — falls back to 2121 and switches it
**on**, which given what that port serves is worth getting right.
