/**
 * Webhook settings functionality
 */

function toggleLimitField(fieldName: string, enabled: boolean): void {
  const field = document.querySelector<HTMLInputElement>(`input[name="${fieldName}"]`);
  if (field) {
    field.disabled = !enabled;
  }
}

export function initRepoSettingsWebhook(): void {
  if (!document.querySelector('.page-content.repository.settings.webhook')) return;

  // Add event listeners for payload optimization checkboxes
  const filesEnableCheckbox = document.querySelector<HTMLInputElement>('input[name="payload_optimization_files_enable"]');
  const commitsEnableCheckbox = document.querySelector<HTMLInputElement>('input[name="payload_optimization_commits_enable"]');

  if (filesEnableCheckbox) {
    filesEnableCheckbox.addEventListener('change', (e) => {
      const target = e.target as HTMLInputElement;
      toggleLimitField('payload_optimization_files_limit', target.checked);
    });
  }

  if (commitsEnableCheckbox) {
    commitsEnableCheckbox.addEventListener('change', (e) => {
      const target = e.target as HTMLInputElement;
      toggleLimitField('payload_optimization_commits_limit', target.checked);
    });
  }
}
