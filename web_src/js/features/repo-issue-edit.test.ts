import {tryQuoteSelectedReply} from './repo-issue-edit.ts';

test('tryQuoteSelectedReply', () => {
  const container = document.createElement('div');
  container.innerHTML = `
    <div class="comment-container">
      <button data-shortcut-handler="quoteSelectedReply">Quote reply</button>
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
  expect(tryQuoteSelectedReply()).toBe(false);

  selection.selectAllChildren(container.querySelector('.raw-content')!);
  expect(tryQuoteSelectedReply()).toBe(false);

  const range = document.createRange();
  range.selectNodeContents(container.querySelector('.render-content')!.firstChild!);
  selection.removeAllRanges();
  selection.addRange(range);
  selection.collapseToStart();
  expect(tryQuoteSelectedReply()).toBe(false);
  expect(onClick).not.toHaveBeenCalled();

  selection.removeAllRanges();
  selection.addRange(range);
  expect(tryQuoteSelectedReply()).toBe(true);
  expect(onClick).toHaveBeenCalledTimes(1);

  button.remove();
  expect(tryQuoteSelectedReply()).toBe(false);
});
