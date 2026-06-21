# Version & Release Harness

## Where the version actually comes from (read this first)

The version string baked into a **released** binary is injected from the **git
tag**, not from this file:

```
git tag v0.2.10
  └─ GoReleaser resolves {{.Version}} from the tag  ->  "0.2.10"
       └─ .goreleaser.yaml:  -X main.version={{.Version}}   (linker -ldflags)
            └─ cmd/statusline/main.go:  var version = "dev"   <- overwritten at link time
                 └─ `statusline --version`  ->  "statusline version 0.2.10 (commit: <sha>)"
```

Consequences — internalize these before touching a release:

- A plain `go build` / `make build` does **not** set the version. The resulting
  binary reports `version dev (commit: unknown)`. Only a GoReleaser release
  build (CI, triggered by pushing a `v*` tag) injects the real value.
- `VERSION`, `.claude-plugin/plugin.json`, and `.claude-plugin/marketplace.json`
  are **metadata only** — marketplace/npm listing plus human reference. None of
  them is read by the binary at runtime or compiled in. They exist so the
  published listing matches the tag.
- The **git tag is the single source of truth** for the shipped version. The
  files below only have to be kept consistent with it: the tag `vX.Y.Z` must
  equal the `X.Y.Z` written into each file.

(Do not confuse this with the `vX.Y.Z` Claude Code version shown *inside* the
statusline output — that one comes from the stdin `version` field /
`claude --version` and is handled by `content/version.go`. Unrelated to this
plugin's own release version.)

## Files to update when bumping version

- `VERSION` — single-line version number
- `.claude-plugin/plugin.json` — `"version"` field
- `.claude-plugin/marketplace.json` — `"version"` field (appears **twice**:
  `metadata.version` and `plugins[0].version`)
- `CHANGELOG.md` — add a new `## [x.y.z] - YYYY-MM-DD` section at the top
  (keep prior entries; bilingual EN + 中文 bullets per existing style)

## Release procedure (order matters)

1. Land the code change first as its own `fix(...)` / `feat(...)` commit.
2. Bump every file in the checklist above to the new `X.Y.Z`.
3. Commit the bump as `chore(release): vX.Y.Z`.
4. Tag it: `git tag vX.Y.Z` — the tag value is what GoReleaser ships.
5. Push both: `git push origin main && git push origin vX.Y.Z`.
6. The pushed tag triggers `.github/workflows/release.yml` -> GoReleaser builds
   every platform and publishes the GitHub Release.

> WARNING: do **not** run `make release-patch` / `release-minor` / `release`
> after you have already hand-edited `VERSION`. Those targets bump `VERSION`
> again (e.g. 0.2.10 -> 0.2.11), commit, and tag that. If the files are already
> bumped, just `git tag vX.Y.Z` matching them and push.
