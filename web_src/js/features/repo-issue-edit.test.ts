import {initRepoIssueCommentEdit} from './repo-issue-edit.ts';

type EditorStub = {value: (v?: string) => string, focus: () => void, moveCursorToEnd: () => void};

initRepoIssueCommentEdit();

function mockEditor(container: HTMLElement): EditorStub {
  let content = '';
  const editor: EditorStub = {
    value(v?: string) {
      if (v !== undefined) content = v;
      return content;
    },
    focus() {},
    moveCursorToEnd() {},
  };
  Object.assign(container, {_giteaComboMarkdownEditor: editor});
  return editor;
}

function commentHtml(id: string, text: string): string {
  return `<div class="timeline-item comment" id="issuecomment-${id}">
    <div class="ui attached segment comment-body">
      <div class="render-content markup"><p>${text}</p></div>
      <div id="issuecomment-${id}-raw" class="raw-content tw-hidden">${text}</div>
    </div>
  </div>`;
}

function setupIssuePage(): EditorStub {
  document.body.innerHTML = `<div class="page-content repository view issue">
    <div class="comment-list">
      ${commentHtml('1', 'first comment')}
      ${commentHtml('2', 'second comment')}
      <form id="comment-form"><div class="combo-markdown-editor"></div></form>
    </div>
  </div>`;
  return mockEditor(document.querySelector('#comment-form .combo-markdown-editor')!);
}

function textNodeOf(commentId: string): Node {
  return document.querySelector(`#issuecomment-${commentId} .render-content.markup p`)!.firstChild!;
}

function selectRange(start: Node, startOffset: number, end: Node, endOffset: number) {
  const range = document.createRange();
  range.setStart(start, startOffset);
  range.setEnd(end, endOffset);
  const selection = window.getSelection()!;
  selection.removeAllRanges();
  selection.addRange(range);
}

async function pressR() {
  document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'r', bubbles: true}));
  await Promise.resolve();
}

test('quote reply shortcut quotes the selected text', async () => {
  const editor = setupIssuePage();
  const node = textNodeOf('2');
  selectRange(node, 0, node, 6);
  await pressR();
  expect(editor.value()).toBe('> second\n\n');

  // a second quote appends below the existing content
  selectRange(textNodeOf('1'), 0, textNodeOf('1'), 5);
  await pressR();
  expect(editor.value()).toBe('> second\n\n\n\n> first\n\n');
});

test('quote reply shortcut ignores unusable selections', async () => {
  const editor = setupIssuePage();

  window.getSelection()!.removeAllRanges();
  await pressR();
  expect(editor.value()).toBe('');

  // a caret with no selected text
  selectRange(textNodeOf('1'), 3, textNodeOf('1'), 3);
  await pressR();
  expect(editor.value()).toBe('');

  // spanning two comments, so no single comment can be quoted
  selectRange(textNodeOf('1'), 0, textNodeOf('2'), 6);
  await pressR();
  expect(editor.value()).toBe('');
});

test('quote reply shortcut ignores modifiers and text inputs', async () => {
  const editor = setupIssuePage();
  selectRange(textNodeOf('1'), 0, textNodeOf('1'), 5);

  document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'r', ctrlKey: true, bubbles: true}));
  await Promise.resolve();
  expect(editor.value()).toBe('');

  const input = document.createElement('input');
  document.body.append(input);
  input.dispatchEvent(new KeyboardEvent('keydown', {key: 'r', bubbles: true}));
  await Promise.resolve();
  expect(editor.value()).toBe('');
});

test('quote reply shortcut is a no-op without a comment form', async () => {
  const editor = setupIssuePage();
  document.querySelector('#comment-form')!.removeAttribute('id'); // anonymous user, or archived/locked repo

  selectRange(textNodeOf('1'), 0, textNodeOf('1'), 5);
  await pressR();
  await new Promise((resolve) => setTimeout(resolve, 0)); // an unguarded editor rejects here, failing the run
  expect(editor.value()).toBe('');
});

test('quote reply shortcut is a no-op in a code comment with no reply button', async () => {
  const mainEditor = setupIssuePage();
  document.querySelector('.comment-list')!.insertAdjacentHTML('beforeend',
    `<div class="comment-code-cloud">${commentHtml('3', 'code comment')}</div>`); // no reply button

  selectRange(textNodeOf('3'), 0, textNodeOf('3'), 4);
  await pressR();
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(mainEditor.value()).toBe(''); // must not fall through to the main comment form
});
