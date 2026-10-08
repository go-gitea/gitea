import {tryShortcutQuoteReply} from './repo-issue-edit.ts';

test('tryShortcutQuoteReply', () => {
  const container = document.createElement('div');
  container.innerHTML = `
    <div class="comment-container">
      <button class="quote-reply">Quote reply</button>
      <div class="render-content markup">Comment text</div>
      <div class="raw-content">Raw text</div>
    </div>
  `;
  document.body.append(container);
  const selection = window.getSelection()!;
  onTestFinished(() => {
    selection.removeAllRanges();
    container.remove();
  });
  const button = container.querySelector('button')!;
  const onClick = vi.fn();
  button.addEventListener('click', onClick);

  selection.removeAllRanges();
  expect(tryShortcutQuoteReply()).toBe(false);

  selection.selectAllChildren(container.querySelector('.raw-content')!);
  expect(tryShortcutQuoteReply()).toBe(false);

  const range = document.createRange();
  range.selectNodeContents(container.querySelector('.render-content')!.firstChild!);
  selection.removeAllRanges();
  selection.addRange(range);
  selection.collapseToStart();
  expect(tryShortcutQuoteReply()).toBe(false);
  expect(onClick).not.toHaveBeenCalled();

  selection.removeAllRanges();
  selection.addRange(range);
  expect(tryShortcutQuoteReply()).toBe(true);
  expect(onClick).toHaveBeenCalledTimes(1);

  button.remove();
  expect(tryShortcutQuoteReply()).toBe(false);
});
