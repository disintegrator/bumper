---
bumper: patch
---

Fixed two crashes in bump provenance resolution: running `bumper commit` or `bumper next` in a directory that is not a git repository now completes with a warning (changelog entries are written without commit prefixes), and a bump file introduced in the repository's root commit now resolves to that commit. The shallow-clone deepening loop also no longer re-walks bump files that were already resolved.
