export function initActionsPermissionsTable(): void {
  const modeRadios = document.querySelectorAll<HTMLInputElement>('.js-permission-mode-radio');
  const permTable = document.querySelector<HTMLTableElement>('table.js-permissions-table');
  const tableSection = document.querySelector<HTMLElement>('#max-permissions-section');
  const followOrgCheckbox = document.querySelector<HTMLInputElement>('.js-follow-org-config');
  const modeSection = document.querySelector<HTMLElement>('.js-permission-mode-section');

  if (!modeRadios.length) return;

  function updateTableState(): void {
    const followOrg = followOrgCheckbox?.checked ?? false;
    const selectedMode = document.querySelector<HTMLInputElement>('input[name="token_permission_mode"]:checked');
    const isCustom = selectedMode?.value === 'custom';

    // Disable entire form when following org config
    for (const radio of modeRadios) {
      radio.disabled = followOrg;
    }

    if (modeSection) {
      modeSection.style.opacity = followOrg ? '0.5' : '1';
    }

    // Disable table if not custom OR following org
    const tableDisabled = !isCustom || followOrg;
    if (permTable) {
      const inputs = permTable.querySelectorAll<HTMLInputElement>('input[type="radio"]');
      for (const input of inputs) {
        input.disabled = tableDisabled;
      }
      permTable.style.display = tableDisabled ? 'none' : '';
    }

    if (tableSection) {
      tableSection.style.display = tableDisabled ? 'none' : '';
    }
  }

  for (const radio of modeRadios) {
    radio.addEventListener('change', updateTableState);
  }

  followOrgCheckbox?.addEventListener('change', updateTableState);

  updateTableState();

  // Cross-Repo Access Table Toggle
  const crossRepoRadios = document.querySelectorAll<HTMLInputElement>('.js-cross-repo-mode');
  const allowedReposSection = document.querySelector<HTMLElement>('#allowed-repos-section');

  if (crossRepoRadios.length && allowedReposSection) {
    function updateCrossRepoState(): void {
      const selectedMode = document.querySelector<HTMLInputElement>('input[name="cross_repo_mode"]:checked');
      const isSelected = selectedMode?.value === 'selected';
      if (allowedReposSection) {
        allowedReposSection.style.display = isSelected ? '' : 'none';
      }
    }

    for (const radio of crossRepoRadios) {
      radio.addEventListener('change', updateCrossRepoState);
    }
    updateCrossRepoState();
  }
}
