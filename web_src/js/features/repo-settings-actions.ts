import {reinitializeAreYouSure} from '../vendor/jquery.are-you-sure.ts';

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
      modeSection.classList.toggle('tw-opacity-50', shouldDisable);
    }

    if (enableMaxCheckbox) {
      enableMaxCheckbox.disabled = shouldDisable;
    }

    if (tableSection) {
      tableSection.classList.toggle('tw-opacity-50', shouldDisable);
    }

    // Disable table if layout is disabled OR max permissions not enabled
    const isMaxEnabled = enableMaxCheckbox ? enableMaxCheckbox.checked : false;
    const tableDisabled = shouldDisable || !isMaxEnabled;

    if (permTable) {
      const inputs = permTable.querySelectorAll<HTMLInputElement>('input[type="radio"]');
      for (const input of inputs) {
        input.disabled = tableDisabled;
      }
      if (isMaxEnabled) {
        permTable.classList.remove('tw-hidden');
      } else {
        permTable.classList.add('tw-hidden');
      }
      permTable.classList.toggle('tw-opacity-50', shouldDisable);
    }
  }

  for (const radio of modeRadios) {
    radio.addEventListener('change', updateTableState);
  }

  overrideOrgCheckbox?.addEventListener('change', updateTableState);
  enableMaxCheckbox?.addEventListener('change', updateTableState);

  updateTableState();
  const form = modeRadios[0].closest('form');
  if (form) {
    reinitializeAreYouSure(form);
  }

  // Cross-Repo Access Table Toggle
  const crossRepoRadios = document.querySelectorAll<HTMLInputElement>('.js-cross-repo-mode');
  const allowedReposSection = document.querySelector<HTMLElement>('#allowed-repos-section');

  if (crossRepoRadios.length && allowedReposSection) {
    function updateCrossRepoState(): void {
      const selectedMode = document.querySelector<HTMLInputElement>('input[name="cross_repo_mode"]:checked');
      const isSelected = selectedMode?.value === 'selected';
      if (isSelected) {
        allowedReposSection!.classList.remove('tw-hidden');
      } else {
        allowedReposSection!.classList.add('tw-hidden');
      }
    }

    for (const radio of crossRepoRadios) {
      radio.addEventListener('change', updateCrossRepoState);
    }
    updateCrossRepoState();
  }
}
