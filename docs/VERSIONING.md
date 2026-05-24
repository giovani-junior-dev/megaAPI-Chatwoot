# Versioning Policy

chatwoot-megaapi-bridge follows [Semantic Versioning 2.0.0](https://semver.org/).
Releases are tagged in git as `vMAJOR.MINOR.PATCH` and published as Docker images
with matching tags.

## SemVer Contract

Given `vMAJOR.MINOR.PATCH`:

- **MAJOR** — incompatible changes to any of:
  - HTTP request/response shapes on `/v1/wa/{slug}` or `/v1/cw/{slug}`.
  - Database schema migrations that are NOT forward-compatible with the previous
    minor version.
  - Environment variables that are renamed or whose meaning changes.
  - Tenant-facing CLI flags (`bridge tenant`, `bridge migrate`, `bridge serve`).
- **MINOR** — backwards-compatible additions:
  - New routes, new metrics, new CLI subcommands, new env vars (with sane defaults).
  - Forward-compatible migrations (additive columns, new indexes, new tables).
  - Performance or reliability improvements that do not change the contract.
- **PATCH** — bug fixes and internal refactors that change no public surface.

What is **not** part of the SemVer contract:
- Internal package layout under `internal/bridge/` — refactoring freely allowed.
- Log line formatting and field ordering — structured JSON keys ARE stable;
  free-form `message` strings are not.
- Deploy template contents (`deploy/templates/*`) — operators must re-render
  on upgrade.

## Deprecation Cycle

A deprecation must be announced at least one MINOR release before removal. The
process:

1. **Mark deprecated** in the same release where the replacement ships.
   - Add a `Deprecated:` Godoc comment.
   - Log a one-shot warning at startup when the deprecated knob is in use.
   - Add an entry under "Deprecated" in `CHANGELOG.md`.
2. **Keep working** for the rest of the current MAJOR series.
3. **Remove** only at the next MAJOR bump, listed under "Removed" in CHANGELOG.

Example timeline for an env var rename:

- `v1.2.0` — introduce `BRIDGE_DB_DSN`, accept old `DATABASE_URL` with warning.
- `v1.3.0` — old name still works; warning continues.
- `v2.0.0` — old name removed.

## Breaking Change Checklist

Before merging a change that breaks the SemVer contract:

- [ ] Documented in `CHANGELOG.md` under "BREAKING CHANGES" for the next major.
- [ ] Migration path documented in `docs/release/RELEASE_NOTES.md` (target release).
- [ ] If env var/CLI flag is renamed: old name accepted with deprecation warning
      for at least one minor release before this major.
- [ ] If DB schema change is destructive: backup-before-migrate logic in
      `deploy/upgrade.sh` covers the new state.
- [ ] If HTTP shape changed: a compat shim is in place OR all known consumers
      (megaAPI webhook config, Chatwoot webhook config) have been notified.
- [ ] CI runs the previous minor's contract tests against the new code.

A PR that breaks the contract without ticking every box must NOT merge.

## Supported Versions

| Version line | Status | Security fixes | Bug fixes |
|---|---|---|---|
| 1.0.x | Current | yes | yes |
| 0.x | EOL | no | no |

Policy: only the latest MAJOR is supported. Operators on an EOL version must
upgrade to receive any fix — no backports.

When a new MAJOR ships, the previous MAJOR gets six months of security-only
support before being declared EOL.

## Pre-release Identifiers

- `vX.Y.Z-rc.N` — release candidate; same contract as the target, with
  potentially-changing internals.
- `vX.Y.Z-alpha.N` / `-beta.N` — feature previews; SemVer contract NOT enforced
  between successive pre-releases of the same target.

Pre-releases ship with the `:rc`, `:alpha`, or `:beta` Docker tag, never `:latest`.

## Release Cadence

- PATCH: as needed, typically within a week of bug discovery.
- MINOR: roughly monthly, batched after feature epics close in beads.
- MAJOR: planned milestones only; coordinated with at least one full quarter
  of notice on the project mailing list.

## Tagging Procedure

```bash
git checkout master
git pull
make lint test
make build
git tag -a vX.Y.Z -m "Release vX.Y.Z"
# push only after CHANGELOG, RELEASE_NOTES, and PR review are complete
git push origin vX.Y.Z
```

Tags are annotated (`-a`) so they carry author identity and a meaningful message
for `git log --tags`. Tags MUST NOT be moved once pushed — re-tag as a new
PATCH instead.

## Container Image Tags

Each release publishes:

- `ghcr.io/.../bridge:vX.Y.Z` — exact release.
- `ghcr.io/.../bridge:vX.Y` — latest patch in that minor line.
- `ghcr.io/.../bridge:vX` — latest in that major line.
- `ghcr.io/.../bridge:latest` — latest stable (NEVER a pre-release).

Operators are encouraged to pin to `:vX.Y` for security patch automation while
retaining minor-version stability.

## Database Migration Compatibility

Within a MAJOR series, the bridge binary at version `vX.Y` MUST be able to
operate against a database migrated by ANY `vX.Z` where `Z<=Y`. The standard
upgrade flow is:

1. Pull new image.
2. Run `bridge migrate` (forward-compatible by contract for same MAJOR).
3. Restart bridge.

For MAJOR upgrades, follow the release notes for that version — destructive
migrations may require downtime.
