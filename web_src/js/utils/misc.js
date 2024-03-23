// returns a file's path from repo root, including a leading slash
export function getFileViewFilePath() {
  const pathWithRepo = document.querySelector('.repo-path')?.textContent?.trim();
  return `/${pathWithRepo.split('/').filter((_, i) => i !== 0).join('/')}`;
}

// returns a file's text content
export function getFileViewFileText() {
  const lineEls = document.querySelectorAll('.file-view .lines-code');
  return Array.from(lineEls, (el) => el.textContent).join('');
}

// create a link with suitable attributes. `props` is a object of props with these additional options:
// - `external`: whether the link is external and should open in new tab
// remarks:
// - no `noopener` attribute for external because browser defaults to it with target `_blank`
// - no `noreferrer` attribute for external because we use `<meta name="referrer" content="no-referrer">`
export function createLink(props = {}) {
  const a = document.createElement('a');

  if (props.external) {
    delete props.external;
    a.target = '_blank';
    a.rel = 'nofollow';
  }

  for (const [key, value] of Object.entries(props)) {
    a[key] = value;
  }

  return a;
}
