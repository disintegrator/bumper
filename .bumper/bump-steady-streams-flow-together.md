---
bumper: patch
---

`bumper builtins cat:default` now defaults to `CHANGELOG.md` in the workspace root, matching `amendlog:default`. Previously it resolved the default path relative to the process working directory, so reading release notes from a subdirectory of the workspace failed to find the changelog that amending had written at the root.
