import {initGlobalSelectorObserver} from '../modules/observer.ts';
import {ComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {
  codeSuggestionInsertion, initCodeCommentEditor, initRepoCodeSuggestions, refreshCodeSuggestionControls,
} from './repo-code-suggestion.ts';

test.each([
  [
    'wrap embedded backticks and replace an inline selection', 'Before selected after', 15, 'first\n  second with ```',
    '\n````suggestion\nfirst\n  second with ```\n````\n', 46,
  ],
  ['preserve a blank source line and reuse surrounding newlines', 'Before\n\nAfter', 7, '', '```suggestion\n\n```', 21],
] as const)('%s', (_name, value, end, source, want, cursor) => {
  expect(codeSuggestionInsertion(value, 7, end, source)).toEqual({text: want, cursor});
});

test('moving a draft between old-side and new-side ranges updates both suggestion toolbars', () => {
  document.body.innerHTML = `<form>
    <input name="start_side" type="hidden" value="previous">
    <div class="combo-markdown-editor">
      <div class="markdown-toolbar-group"><button type="button" data-global-click="onInsertCodeSuggestion"></button></div>
      <div class="editor-toolbar"><button class="gitea-extra-toolbar-button"></button><i class="separator"></i></div>
    </div>
  </form>`;
  onTestFinished(() => document.body.replaceChildren());
  const form = document.querySelector('form')!;
  const editor = new ComboMarkdownEditor(form.querySelector('.combo-markdown-editor')!);
  const startSide = form.querySelector<HTMLInputElement>('input')!;
  const suggestionActions = form.querySelectorAll('.markdown-toolbar-group, .gitea-extra-toolbar-button, .separator');
  expect(initCodeCommentEditor(form)).toBe(editor);
  expect([...suggestionActions].every((action) => action.classList.contains('tw-hidden'))).toBe(true);
  startSide.value = 'proposed';
  expect(initCodeCommentEditor(form)).toBe(editor);
  expect(form.querySelector('.tw-hidden')).toBeNull();
});

test('batches disjoint suggestions, rejects overlap and evicts changed or removed suggestions', async () => {
  const toggleSelector = '.code-suggestion-batch-toggle';
  const controls = (id: number, start: number, end: number, version = 1, path = 'file.go') => `
    <div class="code-suggestion-controls" data-global-init="initCodeSuggestionControls" data-comment-id="${id}"
      data-content-version="${version}" data-path="${path}" data-start-line="${start}" data-end-line="${end}">
      <button class="code-suggestion-batch-toggle tw-hidden" data-global-click="toggleCodeSuggestionBatch"></button>
    </div>`;
  document.body.innerHTML = `
    <button id="code-suggestion-batch" class="tw-hidden" data-add-text="Add" data-remove-text="Remove"
      data-overlap-text="Overlapping suggestions" data-modal-comment_ids=""><span class="code-suggestion-batch-count">0</span></button>
    <div class="code-suggestion-actions">${controls(1, 5, 9)}</div>
    ${controls(2, 9, 12)}${controls(3, 10, 12)}${controls(4, 5, 9, 1, 'other.go')}
  `;
  initRepoCodeSuggestions();
  initGlobalSelectorObserver(null);
  const actions = document.querySelector('.code-suggestion-actions')!;
  const batchButton = document.querySelector('#code-suggestion-batch')!;
  const [first, overlapping, adjacent, otherFile] = document.querySelectorAll<HTMLButtonElement>(toggleSelector);
  const payload = () => batchButton.getAttribute('data-modal-comment_ids')!;
  onTestFinished(() => document.body.replaceChildren());

  first.click();
  expect(first.textContent).toBe('Remove');
  overlapping.click();
  expect(document.querySelector('.toast-error')!.textContent).toContain('Overlapping suggestions');
  adjacent.click();
  otherFile.click();
  expect(payload().split(',').sort()).toEqual(['1:1', '3:1', '4:1']);
  adjacent.click();
  otherFile.click();
  expect(payload()).toBe('1:1');

  refreshCodeSuggestionControls(actions, controls(1, 5, 9, 2));
  const refreshed = actions.querySelector<HTMLButtonElement>(toggleSelector)!;
  await vi.waitFor(() => expect(refreshed.textContent).toBe('Add'));
  expect(payload()).toBe('');
  refreshed.click();
  expect(payload()).toBe('1:2');
  refreshCodeSuggestionControls(actions, '');
  expect(payload()).toBe('');
  expect(batchButton.classList.contains('tw-hidden')).toBe(true);
});
