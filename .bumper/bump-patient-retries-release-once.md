---
bumper: patch
---

Retrying `bumper commit` after a partial failure no longer re-releases groups that already succeeded. Commit progress is checkpointed to `.bumper/commit-checkpoint.toml` after each group completes; a retry skips checkpointed groups, so each group's version bump and changelog amendment run exactly once per batch of pending bumps. The checkpoint is removed when the batch completes, and modifying the pending bump files between attempts starts the batch from scratch.
