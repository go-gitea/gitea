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

// create a external link with suitable attributes
// - no `noopener` attribute because browser defaults to it with target `_blank`
// - no `noreferrer` attribute because we use `<meta name="referrer" content="no-referrer">`
export function createExternalLink(props = {}) {
  const a = document.createElement('a');
  a.target = '_blank';
  a.rel = 'nofollow';
  for (const [key, value] of Object.entries(props)) {
    a[key] = value;
  }
  return a;
}
