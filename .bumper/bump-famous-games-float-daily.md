---
bumper: patch
---

Updated the commit command to gradually unshallow a git repo until the initial commit for each bump file is found. This ensures that the history of changes to these files is accurately tracked. This is a best effort approach and any bump files that cannot be resolved will be logged as warnings.
