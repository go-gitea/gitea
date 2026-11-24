import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {showErrorToast} from '../modules/toast.ts';
import {showElem, toggleElem} from '../utils/dom.ts';
import {getComboMarkdownEditor, initComboMarkdownEditor, toggleExtraToolbarButton} from './comp/ComboMarkdownEditor.ts';

export function codeSuggestionInsertion(value: string, start: number, end: number, source: string) {
  let fenceLength = 3;
  for (const [backticks] of source.matchAll(/`+/g)) {
    fenceLength = Math.max(fenceLength, backticks.length + 1);
  }
  const fence = '`'.repeat(fenceLength);
  const prefix = start > 0 && value[start - 1] !== '\n' ? '\n' : '';
  const suffix = end < value.length && value[end] !== '\n' ? '\n' : '';
  const opening = `${prefix}${fence}suggestion\n${source}`;
  return {text: `${opening}\n${fence}${suffix}`, cursor: start + opening.length};
}

function canInsertCodeSuggestion(form: HTMLFormElement) {
  return (form.elements.namedItem('start_side') as HTMLInputElement).value === 'proposed';
}

function codeSuggestionAnchor(form: HTMLFormElement) {
  if (!canInsertCodeSuggestion(form)) return null;
  const input = (name: string) => form.elements.namedItem(name) as HTMLInputElement | null;
  return new URLSearchParams({
    path: input('path')!.value,
    start_line: input('start_line')!.value,
    line: input('line')!.value,
    commit_id: (input('suggestion_commit_id') ?? input('latest_commit_id'))!.value,
  });
}

export function initCodeCommentEditor(form: HTMLFormElement) {
  const container = form.querySelector<HTMLElement>('.combo-markdown-editor')!;
  const button = form.querySelector<HTMLButtonElement>('button[data-global-click="onInsertCodeSuggestion"]');
  if (button) toggleExtraToolbarButton(button, canInsertCodeSuggestion(form));
  return getComboMarkdownEditor(container) ?? initComboMarkdownEditor(container, button ? {
    easyMDEToolbarButton: button,
    appendPreviewData(data) {
      codeSuggestionAnchor(form)?.forEach((value, key) => { data.append(`code_suggestion_${key}`, value) });
    },
  } : undefined);
}

async function onInsertCodeSuggestion(button: HTMLButtonElement) {
  const form = button.form!;
  const anchor = codeSuggestionAnchor(form)?.toString();
  if (!anchor) return;
  const response = await performFetchActionRequest(button, {
    method: 'GET',
    url: `${form.getAttribute('data-code-suggestion-source-url')!}?${anchor}`,
  });
  if (!response) return;
  const source = await response.arrayBuffer();
  if (!button.isConnected || anchor !== codeSuggestionAnchor(form)?.toString()) return;
  const code = new TextDecoder('utf-8', {ignoreBOM: true}).decode(source);
  getComboMarkdownEditor(button.closest('.combo-markdown-editor'))!.replaceSelection((value, start, end) => codeSuggestionInsertion(value, start, end, code));
}

const batch = new Map<string, {path: string; start: number; end: number; version: string}>();

function updateBatch(batchButton: Element) {
  batchButton.setAttribute('data-modal-comment_ids', Array.from(batch, ([id, item]) => `${id}:${item.version}`).join(','));
  batchButton.querySelector('.code-suggestion-batch-count')!.textContent = String(batch.size);
  toggleElem(batchButton, batch.size > 0);
}

function updateBatchToggle(batchButton: Element, toggle: Element, id: string) {
  toggle.textContent = batchButton.getAttribute(batch.has(id) ? 'data-remove-text' : 'data-add-text')!;
}

export function refreshCodeSuggestionControls(actions: Element | null, controlsHTML: string) {
  if (!actions) return;
  const previousId = actions.firstElementChild?.getAttribute('data-comment-id');
  actions.innerHTML = controlsHTML;
  if (previousId && !actions.firstElementChild && batch.delete(previousId)) {
    updateBatch(document.querySelector('#code-suggestion-batch')!);
  }
}

export function initRepoCodeSuggestions() {
  registerGlobalEventFunc('click', 'onInsertCodeSuggestion', onInsertCodeSuggestion);

  registerGlobalInitFunc('initCodeSuggestionControls', (controls) => {
    const batchButton = document.querySelector('#code-suggestion-batch');
    if (!batchButton) return;
    const toggle = controls.querySelector('.code-suggestion-batch-toggle');
    const id = controls.getAttribute('data-comment-id')!;
    const queued = batch.get(id);
    if (queued && (!toggle || queued.version !== controls.getAttribute('data-content-version'))) {
      batch.delete(id);
      updateBatch(batchButton);
    }
    if (!toggle) return;
    updateBatchToggle(batchButton, toggle, id);
    showElem(toggle);
  });

  registerGlobalEventFunc('click', 'toggleCodeSuggestionBatch', (toggle) => {
    const batchButton = document.querySelector('#code-suggestion-batch')!;
    const controls = toggle.closest('.code-suggestion-controls')!;
    const id = controls.getAttribute('data-comment-id')!;
    if (!batch.delete(id)) {
      const item = {
        path: controls.getAttribute('data-path')!,
        start: Number(controls.getAttribute('data-start-line')!),
        end: Number(controls.getAttribute('data-end-line')!),
        version: controls.getAttribute('data-content-version')!,
      };
      if (Array.from(batch.values()).some((other) => item.path === other.path && item.start <= other.end && other.start <= item.end)) {
        showErrorToast(batchButton.getAttribute('data-overlap-text')!);
        return;
      }
      batch.set(id, item);
    }
    updateBatch(batchButton);
    updateBatchToggle(batchButton, toggle, id);
  });
}
