# Staging Changes

Determine which files to commit by inspecting the current git state.

## Procedure

1. Run `git status` to see the working-tree state.
2. **If files are already staged** (changes in the index) — leave the staging area as-is. The user intentionally staged those files.
3. **If nothing is staged** but there are unstaged changes or untracked files — stage everything with `git add -A`.
4. **If the working tree is completely clean** (nothing staged, nothing unstaged, nothing untracked) — report to the user that there is nothing to commit and **stop**.

After this step, confirm which files are staged and ready for commit.
