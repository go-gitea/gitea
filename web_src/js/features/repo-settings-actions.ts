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
      permTable.style.opacity = tableDisabled ? '0.5' : '1';
    }

    if (tableSection) {
      tableSection.style.opacity = tableDisabled ? '0.5' : '1';
    }
  }

  for (const radio of modeRadios) {
    radio.addEventListener('change', updateTableState);
  }

  followOrgCheckbox?.addEventListener('change', updateTableState);

  updateTableState();
}
