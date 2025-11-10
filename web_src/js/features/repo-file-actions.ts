// Handle repository file/directory actions dropdown
export function initRepoFileActions() {
  const centerContentCheckbox = document.querySelector<HTMLInputElement>('#center-content-checkbox');
  if (!centerContentCheckbox) return;

  // Load saved preference
  const isCentered = localStorage.getItem('repo-content-centered') === 'true';
  centerContentCheckbox.checked = isCentered;
  applyCenterContent(isCentered);

  // Handle checkbox change
  centerContentCheckbox.addEventListener('change', () => {
    const centered = centerContentCheckbox.checked;
    localStorage.setItem('repo-content-centered', String(centered));
    applyCenterContent(centered);
  });
}

function applyCenterContent(centered: boolean) {
  const container = document.querySelector('.ui.container');
  if (!container) return;

  if (centered) {
    container.classList.add('repo-content-centered');
  } else {
    container.classList.remove('repo-content-centered');
  }
}
