import {POST} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {addDelegatedEventListener, toggleElem} from '../utils/dom.ts';
import {debounce} from '../utils/func.ts';
import {attachSearchUserBox} from './comp/SearchUserBox.ts';
import {attachSearchTeamBox} from './repo-settings.ts';

export function parseCvssVector(vector: string, prefix: string): Map<string, string> {
  vector = vector.trim();
  const metrics = new Map<string, string>();
  if (!vector.startsWith(prefix)) return metrics;
  for (const part of vector.substring(prefix.length).split('/')) {
    const [key, value] = part.split(':');
    if (key && value) metrics.set(key, value);
  }
  return metrics;
}

export function composeCvssVector(prefix: string, baseKeys: Array<string>, metrics: Map<string, string>): string {
  if (baseKeys.some((key) => !metrics.get(key))) return '';
  const parts = baseKeys.map((key) => `${key}:${metrics.get(key)}`);
  for (const [key, value] of metrics) {
    if (!baseKeys.includes(key)) parts.push(`${key}:${value}`);
  }
  return `${prefix}${parts.join('/')}`;
}

function initCvssCalculator(container: HTMLElement, severity: HTMLInputElement) {
  const version = container.getAttribute('data-cvss-version')!;
  const prefix = `CVSS:${version}/`;
  const input = container.querySelector<HTMLInputElement>('input')!;
  const result = container.querySelector<HTMLElement>('[data-cvss-result]')!;
  const emptyResult = result.textContent;
  const groups = Array.from(container.querySelectorAll('[data-cvss-metric]'), (group) => [group.getAttribute('data-cvss-metric')!, group] as const);
  const baseKeys = groups.map(([key]) => key);

  const syncMode = () => {
    const selected = severity.value === `cvss:${version}`;
    toggleElem(container, selected);
    input.disabled = !selected;
  };
  severity.addEventListener('change', syncMode);
  syncMode();

  const selectButton = (btn: Element, selected: boolean) => {
    btn.classList.toggle('active', selected);
    btn.setAttribute('aria-pressed', String(selected));
  };

  const updatePreview = debounce(async () => {
    const vector = input.value.trim();
    result.classList.remove('red');
    if (!vector) {
      result.textContent = emptyResult;
      return;
    }
    const data = new FormData();
    data.append('vector', vector);
    const resp = await POST(container.getAttribute('data-preview-url')!, {data});
    if (!resp.ok) return;
    const json = await resp.json();
    if (input.value.trim() !== vector) return;
    result.classList.toggle('red', Boolean(json.error));
    result.textContent = json.error ?? `${json.score.toFixed(1)} ${json.severity}`;
  }, 300);

  const syncButtons = () => {
    const metrics = parseCvssVector(input.value, prefix);
    for (const [key, group] of groups) {
      const value = metrics.get(key);
      for (const btn of group.querySelectorAll('.button')) selectButton(btn, btn.getAttribute('data-value') === value);
    }
  };

  addDelegatedEventListener(container, 'click', '[data-cvss-metric] .button', (btn) => {
    for (const sibling of btn.parentElement!.children) selectButton(sibling, sibling === btn);
    const metrics = parseCvssVector(input.value, prefix);
    for (const [key, group] of groups) {
      const active = group.querySelector('.button.active');
      if (active) metrics.set(key, active.getAttribute('data-value')!);
    }
    const vector = composeCvssVector(prefix, baseKeys, metrics);
    if (!vector) return;
    input.value = vector;
    input.dispatchEvent(new Event('input', {bubbles: true}));
  });
  input.addEventListener('input', () => {
    syncButtons();
    updatePreview();
  });
  syncButtons();
  if (input.value) updatePreview();
}

export function initSecurityAdvisoryForm(form: HTMLFormElement) {
  const severity = form.querySelector<HTMLInputElement>('input[name="severity"]')!;
  for (const calculator of form.querySelectorAll<HTMLElement>('[data-cvss-version]')) initCvssCalculator(calculator, severity);
  addDelegatedEventListener(form, 'click', '[data-remove-row]', (btn) => {
    btn.closest('.security-advisory-row')!.remove();
  });
  addDelegatedEventListener(form, 'click', '[data-add-row]', (btn) => {
    const name = btn.getAttribute('data-add-row')!;
    const template = form.querySelector<HTMLTemplateElement>(`template[data-row-template="${CSS.escape(name)}"]`)!;
    form.querySelector(`[data-rows="${CSS.escape(name)}"]`)!.append(template.content.cloneNode(true));
  });
}

export function initRepoSecurityAdvisory() {
  registerGlobalInitFunc('initSecurityAdvisoryForm', initSecurityAdvisoryForm);
  registerGlobalInitFunc('initSecurityAdvisoryCloseForm', (form: HTMLFormElement) => {
    const originalField = form.querySelector<HTMLElement>('[data-close-duplicate-field]')!;
    form.addEventListener('change', (event) => {
      const radio = event.target as HTMLInputElement;
      if (radio.name === 'close_reason') toggleElem(originalField, radio.value === 'duplicate');
    });
  });
  registerGlobalInitFunc('initSecurityAdvisoryConfidentialQuote', (item) => {
    item.addEventListener('click', () => {
      document.querySelector<HTMLInputElement>('#advisory-comment-confidential')!.checked = true;
    });
  });
  registerGlobalInitFunc('initSecurityAdvisoryUserSearch', attachSearchUserBox);
  registerGlobalInitFunc('initSecurityAdvisoryTeamSearch', attachSearchTeamBox);
}
