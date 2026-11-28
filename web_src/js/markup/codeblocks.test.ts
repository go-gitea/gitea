import {initMarkupCodeBlocks} from './codeblocks.ts';
import {initMarkupCodeCopy} from './codecopy.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {createElementFromHTML} from '../utils/dom.ts';

afterEach(() => vi.restoreAllMocks());

test.each([true, false])('code block controls stay outside the scrolling pre, existing container: %s', (hasContainer) => {
  vi.spyOn(localUserSettings, 'getBoolean').mockReturnValue(false);
  vi.spyOn(localUserSettings, 'setBoolean').mockImplementation(() => {});
  const markup = createElementFromHTML<HTMLElement>(`<div class="markup">
    ${hasContainer ? '<div class="code-block-container code-overflow-scroll">' : ''}
    <pre class="code-block"><code>code</code></pre>
    ${hasContainer ? '</div>' : ''}
  </div>`);
  initMarkupCodeBlocks(markup);
  initMarkupCodeCopy(markup);
  const pre = markup.querySelector('pre')!;
  const button = markup.querySelector<HTMLButtonElement>('.code-wrap')!;
  expect(pre.parentElement!.classList.contains('code-block-container')).toBe(true);
  expect(button.parentElement).toBe(pre.parentElement);
  expect(markup.querySelector('.code-copy')!.parentElement).toBe(pre.parentElement);
  expect(pre.parentElement!.classList.contains('code-overflow-scroll')).toBe(true);
  button.click();
  expect(button.getAttribute('aria-pressed')).toBe('true');
  expect(pre.parentElement!.classList.contains('code-overflow-wrap')).toBe(true);
  expect(pre.parentElement!.classList.contains('code-overflow-scroll')).toBe(false);
  expect(localUserSettings.setBoolean).toHaveBeenCalledWith('wrap-markup-code', true);
});
