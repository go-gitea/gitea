# Releasing Gitea


Release procedure is as follows:

- Let $vmaj, $vmin and $vpat be Major, Minor and Patch version numbers
- Drop "dev" suffix from $vpat
- Let $ver be "$vmaj.$vmin.$vpat"
- Make sure Version variable is set correctly in main.go ($ver)
- Compile CHANGELOG.md section for $ver
  (please only include user-relevant changes, and be concise)
- Commit and push the changelog on both `master` and `release/v$vmaj.$vmin`
- Create PR for changelog
- `git tag -a v$ver`
- If this is a .0 release:
  - `git branch release/v$vmaj.$vmin`
  - Update Version in new branch ( vpat++ )
- Update Version in master branch ( vmin++; vpat=0-dev )
- Push the branches and tags (`git push --tags`)
  - No need to create the Release. CI does that automatically.
- Send PR to https://github.com/go-gitea/blog announcing the release
