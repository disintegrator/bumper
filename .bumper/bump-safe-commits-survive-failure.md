---
bumper: patch
---

`bumper commit` no longer deletes pending bump files before running release group commands. Bump files are now removed only after every group's version and changelog commands have succeeded, so a failure partway through a multi-group release preserves the release intent and the commit can be retried. Failures also report which release group failed.
