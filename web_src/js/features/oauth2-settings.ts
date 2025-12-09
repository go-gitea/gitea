export function initOAuth2SettingsDisableCheckbox() {
  for (const el of document.querySelectorAll<HTMLInputElement>('.disable-setting')) {
    el.addEventListener('change', (e) => {
      const target = e.target as HTMLInputElement;
      const dataTarget = target.getAttribute('data-target')!;
      document.querySelector(dataTarget)!.classList.toggle('disabled', target.checked);
    });
  }
}
