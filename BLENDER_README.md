# Blender Merges

Currently the process for merging upstream changes is to rebase, and keep
Blender modifications on top. This keeps a clear overview of the modifications
that were made.

When merging a major new release, cherry-pick all the Blender commits on
top of it. A simple `git rebase` will not work because the release and main
branches diverge.

First do changes in `blender-merged-develop`, and deploy on uatest. Then apply
the changes in `blender-merged` and deploy in production.
