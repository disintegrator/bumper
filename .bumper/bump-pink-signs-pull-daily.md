---
bumper: minor
---

Added `current:toml`, `next:toml`, `current:json`, `next:json`, `current:yaml` and `next:yaml` builtins that read and update a version string at a dot-separated key path (`--key`) in TOML, JSON and YAML files (`--path`), preserving the surrounding formatting and comments. Useful for manifests like `pyproject.toml`, `Cargo.toml`, `composer.json` or `galaxy.yml`.
