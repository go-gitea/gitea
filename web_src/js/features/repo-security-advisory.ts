import {POST} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';
import {debounce} from '../utils/func.ts';
import {attachSearchUserBox} from './comp/SearchUserBox.ts';
import {attachSearchTeamBox} from './repo-settings.ts';

/** Parses the metrics of a CVSS vector with the given prefix, there are none if the prefix doesn't match */
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

/** Composes a CVSS vector with the base metrics in their order followed by the other metrics, returns '' if a base metric is missing */
export function composeCvssVector(prefix: string, baseKeys: Array<string>, metrics: Map<string, string>): string {
  if (baseKeys.some((key) => !metrics.get(key))) return '';
  const parts = baseKeys.map((key) => `${key}:${metrics.get(key)}`);
  for (const [key, value] of metrics) {
    if (!baseKeys.includes(key)) parts.push(`${key}:${value}`);
  }
  return `${prefix}${parts.join('/')}`;
}

function initCvssCalculator(container: HTMLElement) {
  const prefix = container.getAttribute('data-cvss-prefix')!;
  const input = container.querySelector<HTMLInputElement>('input')!;
  const result = container.querySelector<HTMLElement>('[data-cvss-result]')!;
  const emptyResult = result.textContent;
  const groups = Array.from(container.querySelectorAll<HTMLElement>('[data-cvss-metric]'));
  const baseKeys = groups.map((el) => el.getAttribute('data-cvss-metric')!);

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
    if (!resp.ok || input.value.trim() !== vector) return; // the vector changed while waiting
    const json = await resp.json();
    result.classList.toggle('red', Boolean(json.error));
    result.textContent = json.error ?? `${json.score.toFixed(1)} ${json.severity}`;
  }, 300);

  const syncButtons = () => {
    const metrics = parseCvssVector(input.value, prefix);
    for (const group of groups) {
      const value = metrics.get(group.getAttribute('data-cvss-metric')!);
      for (const btn of group.querySelectorAll('.button')) selectButton(btn, btn.getAttribute('data-value') === value);
    }
  };

  addDelegatedEventListener(container, 'click', '[data-cvss-metric] .button', (btn) => {
    for (const sibling of btn.parentElement!.children) selectButton(sibling, sibling === btn);
    // the vector stays empty until all base metrics are chosen, so the chosen ones are kept by the active buttons
    const metrics = parseCvssVector(input.value, prefix);
    for (const group of groups) {
      const active = group.querySelector('.button.active');
      if (active) metrics.set(group.getAttribute('data-cvss-metric')!, active.getAttribute('data-value')!);
    }
    const vector = composeCvssVector(prefix, baseKeys, metrics);
    if (vector) {
      input.value = vector;
      updatePreview();
    }
  });
  input.addEventListener('input', () => {
    syncButtons();
    updatePreview();
  });
  syncButtons();
  updatePreview();
}

function initRows(form: HTMLFormElement) {
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
  registerGlobalInitFunc('initSecurityAdvisoryForm', (form: HTMLFormElement) => {
    initRows(form);
    for (const el of form.querySelectorAll<HTMLElement>('.security-advisory-cvss')) initCvssCalculator(el);
  });
  registerGlobalInitFunc('initSecurityAdvisoryUserSearch', attachSearchUserBox);
  registerGlobalInitFunc('initSecurityAdvisoryTeamSearch', attachSearchTeamBox);
}
