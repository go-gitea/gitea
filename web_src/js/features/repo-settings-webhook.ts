/**
 * Webhook settings functionality
 */

import {toggleElemClass} from '../utils/dom.ts';

function setupOptimizationToggle(enableFieldName: string, limitFieldName: string): void {
  const enableCheckbox = document.querySelector<HTMLInputElement>(`input[name="${enableFieldName}"]`);
  if (!enableCheckbox) return;

  enableCheckbox.addEventListener('change', (e) => {
    const target = e.target as HTMLInputElement;
    const limitField = document.querySelector<HTMLInputElement>(`input[name="${limitFieldName}"]`);
    if (limitField) {
      limitField.disabled = !target.checked;
      // Use toggleElemClass to show/hide the limit field container
      const limitFieldContainer = limitField.closest('.field');
      if (limitFieldContainer) {
        toggleElemClass(limitFieldContainer, 'tw-hidden', !target.checked);
      }
    }
  });
}

export function initRepoSettingsWebhook(): void {
  if (!document.querySelector('.page-content.repository.settings.webhook')) return;

  // Setup payload optimization toggles
  setupOptimizationToggle('payload_optimization_files_enable', 'payload_optimization_files_limit');
  setupOptimizationToggle('payload_optimization_commits_enable', 'payload_optimization_commits_limit');
}
