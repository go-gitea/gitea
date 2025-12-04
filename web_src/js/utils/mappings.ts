// repo icon, keep in sync with templates/repo/icon.tmpl
export function getRepoIcon(repo: Record<string, any>) {
  if (repo.mirror) {
    return 'octicon-mirror';
  } else if (repo.fork) {
    return 'octicon-repo-forked';
  } else {
    return 'octicon-repo';
  }
}
