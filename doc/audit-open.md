# What the audits have left open

The audits' leftovers that have to outlive them: decisions put to the
maintainer - the code is right and the taste is arguable, a fix that buys
little, or one of the critical decisions `CLAUDE.md` names - and measurements
that reading cannot make. They used to live in `AUDIT_REPORT.md`, a git-ignored
scratchpad that is now started fresh for every audit, so they live here.

An item is added in the commit of the read that raises it, and removed in the
commit that settles it, with the decision in that commit's message.

Carried over on 2026-10-09 from the report of the audits since late September.
Left out as settled by then: D-new-9 (a restart and a cold start give one answer
since the restart hands on the environment the process started with), D-new-26
(the installer's prefill behind the token, #284), D6 (MySQL refuses to start in
another zone, #317), D1 and D2 (no such comments and no such splits remain).
What is below was raised and is not recorded as decided anywhere.

## Decisions waiting on the maintainer

| | Question |
| --- | --- |
| R1 | When do the rename's bridges come down? Since the rename to TimePulse (2026-11-17) every release also attaches its binary as `go-time-recording-web_*` and publishes its image as `ghcr.io/dennis-dko/go-time-recording`, the session and CSRF cookies are written under their `gtr_` names too, and the Android app registers its bridges and dispatches its event under the old names as well - each for a reader older than the writer. Taking them down leaves a binary installation that has not updated since then unable to update itself, a compose file still naming the old image pulling nothing new, and an Android app from before the rename unable to open a session. The old names that are only *read* - token prefixes, the sealed marker, the `GTR_*` variables, the page's storage keys - cost nothing to keep and can stay. CLAUDE.md, section 7, lists all of them. |

## Measurements reading cannot make

Each needs real infrastructure, and none is a change to make on a guess.

- Whether an account can come to depend on a directory entry behind a referral.
  A run collects referrals and does not follow them; `task test:ldap` runs
  OpenLDAP, and this needs a real Active Directory.
- Whether a desktop browser's history keeps the `/?token=` address the
  installer's page was opened with. The page takes it out of the address bar at
  once; headless Chrome records no history, so this needs a real browser.
- The wall time of a first directory run over thousands of accounts on SQLite,
  and on the Pi: one commit per arrival.
