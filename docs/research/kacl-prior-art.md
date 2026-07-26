# Prior art: change categories vs. semver levels in release tools

Research notes for [#75](https://github.com/disintegrator/bumper/issues/75) (map issue
[#74](https://github.com/disintegrator/bumper/issues/74)). This repo has no existing
research-notes convention; `docs/` already holds agent-facing docs, so `docs/research/`
is introduced here as the home for wayfinder research artifacts.

**Question:** how do comparable release tools model change _categories_ (Added/Fixed/
Changed/...) alongside semver _bump levels_ (major/minor/patch)? For each tool: (1) where
the category is captured, (2) whether category and level are coupled or orthogonal,
(3) how categorized entries render into a release section.

## Comparison summary

| Tool                   | Unit of change                               | Category captured where                                                                  | Level captured where                                                                             | Coupled?                                                                                                         | Rendering                                                                                                                                                        |
| ---------------------- | -------------------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Keep a Changelog 1.1.0 | hand-written changelog entry                 | `### <Category>` heading the author files the entry under (6 fixed categories)           | n/a — author picks the version; spec only says "mention whether you follow SemVer"               | Orthogonal (spec is silent on category→level mapping)                                                            | `## [version] - date` then `### Added` / `### Fixed` / ... bullet lists; unused categories simply absent                                                         |
| changesets             | per-change markdown file (`.changeset/*.md`) | **not expressible** — summary is free-form markdown                                      | explicit: YAML frontmatter maps each package to `major`/`minor`/`patch`                          | Level only; no category axis                                                                                     | Grouped by **level**: `## <version>` → `### Major Changes` / `### Minor Changes` / `### Patch Changes`; empty groups filtered out                                |
| towncrier              | per-change news fragment file                | explicit: filename suffix `<issue>.<type>` (e.g. `123.feature`); types defined in config | **not captured** — version is passed in via `--version`/config; towncrier computes nothing       | Category only; no level axis                                                                                     | One section per type, ordered by `[[tool.towncrier.type]]` config order (or alphabetical); `showcontent = false` types render names only; empty sections omitted |
| git-cliff              | conventional commit                          | derived: `commit_parsers` regexes map message patterns to `group` strings                | derived: separate `[bump]` rules (breaking→major, `feat`→minor, else patch, plus custom regexes) | Both derived from the same commit message, but by **independent** config (a group rename never changes the bump) | Template groups commits by `group`; ordering via `<!-- 0 -->` HTML-comment prefixes stripped with `striptags`                                                    |
| cocogitto              | conventional commit                          | commit type; per-type `changelog_title` in `[commit_types]`                              | commit type; per-type `bump_patch` / `bump_minor` flags (breaking `!`/footer → major)            | **Coupled**: one commit type drives both section and bump                                                        | Sections per type in fixed default order (`feat, fix, perf, ...`), customizable via `order`; `omit_from_changelog` hides a type                                  |
| release-please         | conventional commit                          | commit type → `changelog-sections` config (`{type, section, hidden}`)                    | commit type → hardcoded strategy (breaking→major, `feat`→minor, else patch)                      | **Coupled**: one commit type drives both                                                                         | Visible sections in config order (`Features`, `Bug Fixes`, ...); `hidden: true` types dropped unless the commit is breaking                                      |
| changie (bonus)        | per-change YAML fragment file                | explicit: `kind` chosen per change file, kinds defined in `.changie.yaml`                | derived: each kind's `auto` field maps it to major/minor/patch/none; highest wins                | **Category implies level** (configurable mapping; templates can escape it)                                       | `changie batch` groups changes by kind under kind headers in the version file                                                                                    |

**Headline finding:** none of the six required tools carries _both_ an explicit semver
level _and_ an explicit category on each change record. File-based tools pick exactly one
axis (changesets = level only, towncrier = category only); commit-based tools derive both
from a single commit type, coupling them. The closest existing design to bumper's plan is
**changie**: an explicit per-file category with the level _derived from_ the category via
config. Bumper's planned model (explicit levels per group + one explicit KACL category
per bump file, independently chosen) has no direct precedent among these tools.

---

## Keep a Changelog 1.1.0

Source: <https://keepachangelog.com/en/1.1.0/>

- **Categories:** six change types, each defined in one line: **Added** "for new
  features", **Changed** "for changes in existing functionality", **Deprecated** "for
  soon-to-be removed features", **Removed** "for now removed features", **Fixed** "for
  any bug fixes", **Security** "in case of vulnerabilities".
- **Structure:** an `## [Unreleased]` section at the top; version sections newest-first
  as `## [x.y.z] - YYYY-MM-DD` (ISO 8601 dates); within a version, change types appear
  as `###` sub-headings containing bullet lists. "Versions and sections should be
  linkable."
- **Semver interaction:** the spec's own example changelog states it "adheres to
  Semantic Versioning", and the guidance says to "Mention whether you follow Semantic
  Versioning" — but the spec prescribes **no mapping** from change type to bump level.
  Category and version are fully orthogonal in the spec; the version number is whatever
  the author releases.
- **Empty categories:** the spec's examples only include headings that have entries;
  there is no notion of rendering an empty category.

## changesets

Sources:
<https://github.com/changesets/changesets/blob/main/docs/detailed-explanation.md>,
<https://github.com/changesets/changesets/blob/main/docs/modifying-changelog-format.md>,
<https://github.com/changesets/changesets/blob/main/packages/apply-release-plan/src/get-changelog-entry.ts>

- **File format** (nearly identical to bumper's bump files):

  ```md
  ---
  "@myproject/cli": major
  "@myproject/core": minor
  ---

  Change all the things
  ```

  YAML frontmatter maps package names to bump types; the body is a free-form markdown
  summary "which will be written to the changelog".

- **Category (1):** there is **no category concept**. The only classification a
  changeset carries is the bump type per package. Nothing in the file format or config
  expresses Added/Fixed/etc.
- **Coupling (2):** n/a — only the level axis exists.
- **Rendering (3):** `apply-release-plan` assembles each release entry as
  `## ${release.newVersion}` followed by up to three sections generated by
  `generateMarkdownForVersionType`, headed `### ${capitalize(type)} Changes` — i.e.
  `### Major Changes`, `### Minor Changes`, `### Patch Changes`. The changelog's section
  structure is therefore organized **by semver level, not by category**. Empty levels
  are skipped (`if (!releaseLines.length) return;` plus a `.filter((line) => line)`
  before joining).
- **Changelog-generator plugin interface:** `.changeset/config.json`'s `changelog`
  option names a module exporting `getReleaseLine(changeset, type, changelogOpts)` and
  `getDependencyReleaseLine(...)` (types in `@changesets/types` as
  `ChangelogFunctions`). `getReleaseLine` receives the changeset (summary, releases,
  commit) and the bump `type` and returns the formatted line — so a third-party
  generator can restyle _lines_, but the Major/Minor/Patch grouping above is imposed
  outside the plugin. Category support would have to be smuggled through summary-text
  conventions.

## towncrier

Sources: <https://towncrier.readthedocs.io/en/stable/configuration.html>,
<https://towncrier.readthedocs.io/en/stable/cli.html>

- **Category (1):** captured **in the fragment filename**: `<issue>.<type>`, e.g.
  `123.feature`, `456.bugfix`. Default types: `feature`, `bugfix`, `doc`, `removal`,
  `misc`.
- **Types are config-defined:** either `[tool.towncrier.fragment.<type>]` tables
  (rendered alphabetically — "Since TOML mappings aren't ordered, types defined using
  this method are always rendered alphabetically") or ordered `[[tool.towncrier.type]]`
  arrays. Per-type fields: `name` (section heading, defaults to capitalized type),
  `directory` (the filename suffix), `showcontent` (include fragment text or only issue
  numbers; default true), `check` (whether `towncrier check` requires it).
- **Coupling (2):** none — towncrier has **no version computation at all**. The version
  string in the rendered news file comes from the `--version` CLI option ("Use VERSION
  in the rendered news file. Can be configured or guessed") or from project metadata;
  fragment types never influence it. Category only, level external.
- **Rendering (3):** each type becomes a section under the version header, in the order
  types are declared (`[[tool.towncrier.type]]`) or alphabetical. Fragments for the same
  issue are merged; empty type sections are omitted; `showcontent = false` types (e.g.
  `misc`) render as a list of issue links only.

## git-cliff

Sources: <https://git-cliff.org/docs/configuration/git>,
<https://git-cliff.org/docs/configuration/bump>,
<https://github.com/orhun/git-cliff/blob/main/config/cliff.toml>

- **Category (1):** derived from the **commit message** via `commit_parsers` — "an array
  of commit parsers for determining the commit groups by using regex":

  ```toml
  commit_parsers = [
      { message = "^feat", group = "<!-- 0 -->🚀 Features" },
      { message = "^fix",  group = "<!-- 1 -->🐛 Bug Fixes" },
      { message = "^revert", skip = true },
  ]
  ```

  Groups are arbitrary strings; capture groups can be interpolated (`'Fix (${1})'`);
  `skip = true` drops a commit from the changelog entirely.

- **Level (2):** computed by a **separate** `[bump]` rule set over the same commits:
  breaking commits bump major (`breaking_always_bump_major`, "a breaking change commit
  will always trigger a major version update"), features bump minor
  (`features_always_bump_minor`), everything else patch — extensible with
  `custom_major_increment_regex` / `custom_minor_increment_regex`,
  `no_increment_regex`, or forced via `bump_type`. Changelog **groups play no role** in
  bump computation; category and level are parallel derivations from the commit message,
  configured independently (skipped-from-changelog commits can still bump, and renaming
  a group changes nothing about versions).
- **Rendering (3):** the Tera template groups commits by `group` and renders
  `### {{ group | striptags | trim | upper_first }}`. Ordering is controlled by the
  `<!-- 0 -->` / `<!-- 1 -->` HTML-comment prefixes in group names — invisible sort keys
  stripped by `striptags` at render time. Only groups with matched commits appear.

## cocogitto

Sources: <https://docs.cocogitto.io/guide/bump.html>,
<https://docs.cocogitto.io/guide/commit>, <https://docs.cocogitto.io/reference/config.html>

- **Category (1):** the **conventional commit type** is the category. `cog bump --auto`:
  `fix` "correlates with PATCH", `feat` "correlates with MINOR", and a `BREAKING CHANGE:`
  footer or `!` "correlat[es] with MAJOR".
- **Coupling (2):** **tightly coupled, per-type configurable**. `[commit_types]` in
  `cog.toml` is a map of type → config where the _same_ entry controls both axes:
  `bump_patch` / `bump_minor` ("Allow for this commit type to bump the patch/minor
  version") sit next to `changelog_title` ("the title used in generated changelog for
  this commit type") and `omit_from_changelog`:

  ```toml
  [commit_types]
  custom = { changelog_title = "Custom Changes", bump_minor = true }
  test = { omit_from_changelog = true }
  ```

  One commit type simultaneously determines its changelog section and whether it bumps.

- **Rendering (3):** changelog sections are per commit type, "ordered by type feat, fix,
  perf, revert, docs, test, build, ci, refactor, chore, style" by default; the `order`
  field on a commit type customizes placement; `omit_from_changelog` hides a type; the
  `[changelog]` table controls path/template/remote linking.

## release-please

Sources: <https://github.com/googleapis/release-please/blob/main/README.md>,
<https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md>,
<https://github.com/googleapis/release-please/blob/main/src/util/filter-commits.ts>,
<https://github.com/googleapis/release-please/blob/main/src/versioning-strategies/default.ts>,
<https://github.com/googleapis/release-please/blob/main/src/strategies/python.ts>

- **Category (1):** the conventional commit type, mapped to headings by the
  `changelog-sections` config ("set default conventional commit => changelog sections
  mapping/appearance"), an array of `{type, section, hidden}`. Default
  (`DEFAULT_CHANGELOG_SECTIONS` in `src/util/filter-commits.ts`):

  ```ts
  const DEFAULT_CHANGELOG_SECTIONS = [
    { type: "feat", section: "Features" },
    { type: "fix", section: "Bug Fixes" },
    { type: "perf", section: "Performance Improvements" },
    { type: "revert", section: "Reverts" },
    { type: "chore", section: "Miscellaneous Chores", hidden: true },
    { type: "docs", section: "Documentation", hidden: true },
    // style, refactor, test, build, ci — all hidden: true
  ];
  ```

  Per-language strategies override it (e.g. Python un-hides `docs` and adds `deps`).

- **Level (2):** computed by `DefaultVersioningStrategy` from the same commits:
  "Breaking changes should bump the major, features should bump the minor, and other
  significant changes should bump the patch version" — with pre-1.0 modifiers
  (`bumpMinorPreMajor`, `bumpPatchForMinorPreMajor`) and a `Release-As` footer override.
  So type → section and type → level are **two fixed functions of the one commit type**;
  users can remap sections freely but the bump mapping is the strategy's, not the
  section config's.
- **Rendering (3):** visible sections render as headings in `changelog-sections` order;
  `hidden: true` types are filtered out of the changelog — _unless_ the commit is
  breaking, in which case even hidden-type commits are shown
  (`visibleSections.includes(commit.type) || (isBreaking && hiddenSections.includes(...))`).
  Empty sections don't render. Multiple logical changes can ride one commit via extra
  conventional-commit footers, each becoming its own entry.

## Bonus: changie (closest prior art)

Source: <https://changie.dev/config/>

Not in the required list, but the only surveyed tool where a per-change _file_ carries an
explicit category: `.changie.yaml` defines `kinds` (e.g. `label: Added`, `label: Fixed`),
each change fragment records its chosen kind, and `changie batch` groups changes by kind
under kind headers. The semver level is _derived from_ the category: each kind's `auto`
field — "Auto determines what value to bump when using `batch auto` or `next auto`.
Possible values are major, minor, patch or none and the highest one is used if multiple
changes are found" — and it can even be a template over custom prompt fields
(`auto: '{{if eq .Custom.Breaking "Yes"}}major{{else}}patch{{end}}'`). So changie chose
**category implies level** (with a configurable mapping) rather than capturing both
independently.

## Implications for bumper (facts and patterns, not decisions)

- **Bumper's planned shape — explicit level(s) _and_ one explicit KACL category per bump
  file — is novel among these tools.** Every surveyed tool either captures one axis
  explicitly and ignores/externalizes the other (changesets: level; towncrier:
  category), derives both from a single token (commit type: git-cliff, cocogitto,
  release-please), or derives the level from the category (changie).
- **The KACL spec imposes no category→level mapping**, so keeping the two orthogonal in
  bump files does not conflict with the spec; conversely, tools that do couple them
  (cocogitto, changie, release-please) each invented their own mapping and made it
  configurable, suggesting any default coupling bumper offered would need an escape
  hatch.
- **"Highest wins" is the universal squash rule** wherever multiple changes meet one
  release: changesets across changesets per package, release-please/git-cliff/cocogitto
  across commits, changie across kinds' `auto` values — matching bumper's existing
  highest-level-wins squash.
- **Rendering conventions converge:** `##` version heading → `###` category sections;
  fixed, config-declared section order (towncrier's declaration order, cocogitto's
  `order`, release-please's array order, git-cliff's HTML-comment sort keys); empty
  categories always omitted, never rendered as empty headings. Changesets is the outlier
  in grouping by _level_ instead of category.
- **Hidden/suppressed categories are a recurring feature** (towncrier `showcontent`,
  cocogitto `omit_from_changelog`, release-please `hidden` + its breaking-change
  override, git-cliff `skip`): tools distinguish "this change exists and may affect the
  version" from "this change appears in the changelog".
- **Plugin surface caveat:** changesets shows the cost of baking grouping into the core —
  its changelog plugin interface can restyle lines but cannot change the
  Major/Minor/Patch section structure. Bumper's `changelog_cmd` contract (env-var driven
  external command) would need the category passed explicitly (e.g. a `BUMPER_*` env var
  or structured payload) for external generators to group by it.
