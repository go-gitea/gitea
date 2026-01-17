export function initActionsPermissionsTable(): void {
  const modeRadios = document.querySelectorAll<HTMLInputElement>('.js-permission-mode-radio');
  const permTable = document.querySelector<HTMLTableElement>('table.js-permissions-table');
  const tableSection = document.querySelector<HTMLElement>('#max-permissions-section');
  const overrideOrgCheckbox = document.querySelector<HTMLInputElement>('.js-override-org-config');
  const modeSection = document.querySelector<HTMLElement>('.js-permission-mode-section');
  const enableMaxCheckbox = document.querySelector<HTMLInputElement>('.js-enable-max-permissions');

  if (!modeRadios.length) return;

  function updateTableState(): void {
    // If the checkbox exists (Repo settings), we are disabled if it is NOT checked (Follow mode).
    // If the checkbox does not exist (Org settings), we are never disabled by this rule.
    const shouldDisable = overrideOrgCheckbox ? !overrideOrgCheckbox.checked : false;

    // Disable entire form when following org config (Override unchecked)
    for (const radio of modeRadios) {
      radio.disabled = shouldDisable;
    }

    if (modeSection) {
      modeSection.style.opacity = shouldDisable ? '0.5' : '1';
    }

    if (enableMaxCheckbox) {
      enableMaxCheckbox.disabled = shouldDisable;
    }

    if (tableSection) {
      tableSection.style.opacity = shouldDisable ? '0.5' : '1';
    }

    // Disable table if layout is disabled OR max permissions not enabled
    const isMaxEnabled = enableMaxCheckbox ? enableMaxCheckbox.checked : false;
    const tableDisabled = shouldDisable || !isMaxEnabled;

    if (permTable) {
      const inputs = permTable.querySelectorAll<HTMLInputElement>('input[type="radio"]');
      for (const input of inputs) {
        input.disabled = tableDisabled;
      }
      permTable.style.display = isMaxEnabled ? '' : 'none';
      if (shouldDisable) {
        permTable.style.opacity = '0.5';
        // If disabled, we might want to hide it or just show disabled state?
        // If following Org config, the Org might have max permissions set.
        // But here we are configuring the REPO overrides.
        // If not overriding, we show nothing (or disabled state).
        // Current logic dims everything.
      } else {
        permTable.style.opacity = '1';
      }
    }
  }

  for (const radio of modeRadios) {
    radio.addEventListener('change', updateTableState);
  }

  overrideOrgCheckbox?.addEventListener('change', updateTableState);
  enableMaxCheckbox?.addEventListener('change', updateTableState);

  updateTableState();

  // Cross-Repo Access Table Toggle
  const crossRepoRadios = document.querySelectorAll<HTMLInputElement>('.js-cross-repo-mode');
  const allowedReposSection = document.querySelector<HTMLElement>('#allowed-repos-section');

  if (crossRepoRadios.length && allowedReposSection) {
    function updateCrossRepoState(): void {
      const selectedMode = document.querySelector<HTMLInputElement>('input[name="cross_repo_mode"]:checked');
      const isSelected = selectedMode?.value === 'selected';
      allowedReposSection!.style.display = isSelected ? '' : 'none';
    }

    for (const radio of crossRepoRadios) {
      radio.addEventListener('change', updateCrossRepoState);
    }
    updateCrossRepoState();
  }
}
