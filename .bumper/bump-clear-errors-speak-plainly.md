---
bumper: patch
---

Commands now report clear errors where they previously exited silently: an unknown `--group`, a missing `--group` with multiple release groups defined, and a missing or invalid configuration each print a descriptive message. `bumper init` also defaults to the current directory as documented.
