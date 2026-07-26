# bumper

Bumper is a Go CLI for managing versioning and changelogs at scale, in any language or repo shape. Contributors record an "intent to release" as bump files (`.bumper/bump-*.md`, YAML frontmatter mapping release groups to patch/minor/major plus a markdown changelog entry) that merge alongside their PRs. When it's time to release, `bumper commit` squashes pending bumps per release group (highest level wins), computes the next semver, and updates versions and changelogs — not directly, but by shelling out to per-group command arrays (`current_cmd`, `next_cmd`, `changelog_cmd`, `cat_cmd`) defined in `.bumper/config.toml`, communicating via env vars (`BUMPER_GROUP`, `BUMPER_GROUP_NEXT_VERSION`, ...). Defaults point back at `bumper builtins <verb>:<strategy>` (default/file/npm strategies), but any external command honoring the contract can be swapped in. This repo dogfoods itself: its own version lives in `VERSION`, driven by `.bumper/config.toml` and released through GitHub Actions + goreleaser.

## Example usage

```sh
bumper init                        # create .bumper/ in the repo
bumper create api dashboard        # define release groups
bumper bump                        # record a change (interactive; or --group/--minor/-m)
bumper commit                      # squash pending bumps, update versions + changelogs
bumper current --group api         # print current version
bumper next --group api            # preview next version from pending bumps
bumper cat --group api --version 1.2.0   # print that release's changelog section
```

## Development

- `mise run bumper <args>` — build (`go build -o local/bumper ./cmd/bumper`) and run the CLI.
- No tests or lint config exist; use `go build ./...` / `go vet ./...` to verify changes.
- Docs site: `bun run --cwd site dev` (Astro + Starlight). `mise run site:cli-reference` regenerates the CLI reference pages from `--help` output.
- Toolchain is pinned in `mise.toml` (go, bun). Releases: merging a bump file to main opens a release PR (`release-pr.yaml`); merging that PR changes `VERSION`, which tags and runs goreleaser (`release.yaml`).

## Project structure

```
cmd/bumper/main.go       CLI entrypoint: urfave/cli/v3 root command, slog logger, signal handling
internal/commands/       One package per subcommand, each exposing NewCommand(logger)
  initialize, create,    init workspace; define release groups
  bump, commit,          write bump files (huh interactive form); squash + release
  current, next, cat     query versions / changelog sections (exec group commands)
  builtins/              `bumper builtins <verb>:<strategy>` — default (versions.toml +
                         CHANGELOG.md), file (plain version file), npm (package.json)
  shared/                --dir/--group flag helpers, config load/save wrappers
internal/workspace/      Core model: config.toml schema + validation, bump file
                         parsing/aggregation, workspace discovery (walk up to .bumper/),
                         git integration (go-git; finds commit that introduced each bump)
internal/cmd/            CommandError sentinel (cmd.Failed) to avoid double-printing errors
internal/o11y/           slog logger backed by charmbracelet/log, writes to stderr
internal/random/         adjective-noun-verb-adverb slugs for bump filenames
buildinfo/               Version/Commit/Date vars injected via goreleaser ldflags
site/                    Docs site (Astro + Starlight + Tailwind), deployed to
                         bumper.disintegrator.dev; content in site/src/content/docs/
.bumper/                 This repo's own bumper config (group "bumper" → VERSION file)
.mise-tasks/             mise file tasks (build/run, release notes, CLI-reference gen)
.github/workflows/       release-pr, release (goreleaser + cosign), docs deploy
```

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (`disintegrator/bumper`) via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `CONTEXT.md` at the repo root plus `docs/adr/`. See `docs/agents/domain.md`.
