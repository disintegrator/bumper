---
bumper: patch
---

`bumper bump` no longer requires a terminal when all inputs are provided via flags. Invocations like `bumper bump --group api --minor -m "msg"` now work in CI and other non-interactive environments; the interactive form only runs when a group, level, or message still needs to be collected.
