import {initScopedWorkflowRequired} from './ScopedWorkflows.ts';

function setupForm(required = false) {
  window.document.body.innerHTML = `
<form>
  <table><tbody>
    <tr>
      <td>ci.yaml<input type="hidden" name="workflow_ids" value="ci.yaml"></td>
      <td><div class="ui checkbox"><input type="checkbox" class="js-scoped-required-toggle" ${required ? 'checked' : ''}><label></label></div></td>
      <td>
        <textarea class="js-scoped-required-patterns${required ? '' : ' tw-hidden'}" data-default-pattern="org/src: CI / *">${required ? 'org/src: CI / *' : ''}</textarea>
        <span class="js-scoped-required-hint${required ? ' tw-hidden' : ''}">hint</span>
      </td>
    </tr>
  </tbody></table>
</form>`;
  const form = document.querySelector('form')!;
  const checkbox = form.querySelector<HTMLInputElement>('.js-scoped-required-toggle')!;
  const textarea = form.querySelector<HTMLTextAreaElement>('.js-scoped-required-patterns')!;
  const hint = form.querySelector<HTMLElement>('.js-scoped-required-hint')!;
  return {form, checkbox, textarea, hint};
}

test('required toggle shows/prefills the patterns textarea (and hides the hint) and reverses otherwise, keeping the value', () => {
  const {form, checkbox, textarea, hint} = setupForm();
  initScopedWorkflowRequired(form);
  expect(textarea.classList.contains('tw-hidden')).toBe(true); // initial: not required -> textarea hidden
  expect(hint.classList.contains('tw-hidden')).toBe(false); // ... and the hint shown in its place

  // check -> textarea shown and prefilled; hint hidden
  checkbox.checked = true;
  checkbox.dispatchEvent(new Event('change', {bubbles: true}));
  expect(textarea.classList.contains('tw-hidden')).toBe(false);
  expect(hint.classList.contains('tw-hidden')).toBe(true);
  expect(textarea.value).toBe('org/src: CI / *');

  // admin edits the pattern
  textarea.value = 'org/src: CI / build (pull_request)';

  // uncheck -> textarea hidden (value kept, still submits as history), hint shown again
  checkbox.checked = false;
  checkbox.dispatchEvent(new Event('change', {bubbles: true}));
  expect(textarea.classList.contains('tw-hidden')).toBe(true);
  expect(hint.classList.contains('tw-hidden')).toBe(false);
  expect(textarea.value).toBe('org/src: CI / build (pull_request)');

  // re-check -> shown again with the same value (not re-prefilled to the default)
  checkbox.checked = true;
  checkbox.dispatchEvent(new Event('change', {bubbles: true}));
  expect(textarea.classList.contains('tw-hidden')).toBe(false);
  expect(textarea.value).toBe('org/src: CI / build (pull_request)');
});

test('an already-required row stays shown with its stored patterns (not re-prefilled)', () => {
  const {form, textarea} = setupForm(true);
  textarea.value = 'org/src: custom / build (push)'; // a stored, admin-edited pattern
  initScopedWorkflowRequired(form);
  expect(textarea.classList.contains('tw-hidden')).toBe(false);
  expect(textarea.value).toBe('org/src: custom / build (push)');
});

function setupFormWithContexts(patterns: string) {
  window.document.body.innerHTML = `
<form>
  <table><tbody>
    <tr>
      <td>ci.yaml<input type="hidden" name="workflow_ids" value="ci.yaml"></td>
      <td><div class="ui checkbox"><input type="checkbox" class="js-scoped-required-toggle" checked><label></label></div></td>
      <td>
        <textarea class="js-scoped-required-patterns" data-default-pattern="org/src: CI / *">${patterns}</textarea>
        <span class="js-scoped-required-hint tw-hidden">hint</span>
        <table class="js-scoped-required-contexts"><tbody>
          <tr><td><span class="js-scoped-context" data-context="org/src: CI / lint (push)"></span><span class="js-scoped-context-matched tw-hidden">Matched</span></td></tr>
          <tr><td><span class="js-scoped-context" data-context="org/src: CI / build (push)"></span><span class="js-scoped-context-matched tw-hidden">Matched</span></td></tr>
        </tbody></table>
      </td>
    </tr>
  </tbody></table>
</form>`;
  const form = document.querySelector('form')!;
  const [lintMark, buildMark] = Array.from(form.querySelectorAll<HTMLElement>('.js-scoped-context-matched'));
  return {form, lintMark, buildMark};
}

test('an exact pattern marks only the context it matches', () => {
  const {form, lintMark, buildMark} = setupFormWithContexts('org/src: CI / lint (push)');
  initScopedWorkflowRequired(form);
  expect(lintMark.classList.contains('tw-hidden')).toBe(false); // matched
  expect(buildMark.classList.contains('tw-hidden')).toBe(true); // not matched
});

test('a wildcard pattern marks every matching context', () => {
  const {form, lintMark, buildMark} = setupFormWithContexts('org/src: CI / *');
  initScopedWorkflowRequired(form);
  expect(lintMark.classList.contains('tw-hidden')).toBe(false);
  expect(buildMark.classList.contains('tw-hidden')).toBe(false);
});

test('a wildcard crossing "/" matches every matching context', () => {
  const {form, lintMark, buildMark} = setupFormWithContexts('org/src: *');
  initScopedWorkflowRequired(form);
  expect(lintMark.classList.contains('tw-hidden')).toBe(false);
  expect(buildMark.classList.contains('tw-hidden')).toBe(false);
});
