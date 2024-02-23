export function getFileViewFileName() {
  return document.querySelector('.repo-path .active')?.textContent?.trim();
}

export function getFileViewContent() {
  const lineEls = document.querySelectorAll('.file-view .lines-code');
  return Array.from(lineEls, (el) => el.textContent).join('');
}
