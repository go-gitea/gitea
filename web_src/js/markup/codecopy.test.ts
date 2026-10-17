import {initMarkupCodeCopy} from './codecopy.ts';
import {copyToClipboardWithFeedback} from '../modules/clipboard.ts';
import {createElementFromHTML} from '../utils/dom.ts';

vi.mock('../modules/clipboard.ts', () => ({copyToClipboardWithFeedback: vi.fn()}));
vi.mock('../svg.ts', () => ({svg: () => '<svg></svg>'}));

test('copy preserves server-trimmed newlines after replacement and trims updated untrimmed content', () => {
  const markup = createElementFromHTML<HTMLElement>(`<div>
    <div class="code-block-container code-block-trimmed"><pre class="code-block"><code>code\n</code></pre></div>
    <div class="code-block-container"><pre class="code-block"><code>old\n</code></pre></div>
  </div>`);
  initMarkupCodeCopy(markup);

  const [trimmedButton, untrimmedButton] = markup.querySelectorAll<HTMLButtonElement>('.code-copy');
  markup.querySelector('pre')!.replaceWith(document.createElement('iframe'));
  trimmedButton.click();
  expect(copyToClipboardWithFeedback).toHaveBeenLastCalledWith(trimmedButton, 'code\n');

  const untrimmedCode = markup.querySelector('code')!;
  untrimmedCode.textContent = 'updated\n\n';
  untrimmedCode.parentElement!.replaceWith(document.createElement('iframe'));
  untrimmedButton.click();
  expect(copyToClipboardWithFeedback).toHaveBeenLastCalledWith(untrimmedButton, 'updated\n');
});
