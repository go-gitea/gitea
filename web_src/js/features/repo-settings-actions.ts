export function initRepoSettingsActionsPermissions(): void {
  const radios = document.querySelectorAll<HTMLInputElement>(
    'input[name="token_permission_mode"]',
  );
  if (!radios.length) return;

  function toggleCustom(): void {
    const customPerms = document.querySelector<HTMLElement>('#custom-permissions');
    if (!customPerms) return;

    const selected = document.querySelector<HTMLInputElement>(
      'input[name="token_permission_mode"]:checked',
    );

    customPerms.style.display =
      selected?.value === 'custom' ? 'block' : 'none';
  }

  for (const radio of radios) {
    radio.addEventListener('change', toggleCustom);
  }

  toggleCustom();
}
