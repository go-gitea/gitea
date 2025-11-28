export function getRepoIcon(repo: Record<string, any>) {
  if (repo.mirror) {
    return 'octicon-mirror';
  } else if (repo.fork) {
    return 'octicon-repo-forked';
  } else if (repo.private) {
    return 'octicon-repo-locked';
  } else if (repo.template) {
    return `octicon-repo-template`;
  }
  return 'octicon-repo';
}
