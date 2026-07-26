---
bumper: patch
---

Bump files with CRLF line endings or without a trailing newline now parse correctly. Previously a Windows-authored bump file failed with "front matter must start with ---", and a file whose closing `---` lacked a trailing newline corrupted the front matter.
