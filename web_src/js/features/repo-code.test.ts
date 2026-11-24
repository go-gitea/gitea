import {initRepoCodeView} from './repo-code.ts';

const originalUrl = window.location.href;

beforeAll(() => {
  document.body.innerHTML = '<div class="repo-view-container"><div class="file-view"></div></div>';
  initRepoCodeView();
});

afterEach(() => {
  document.querySelector('.code-line-button')?._tippy.destroy();
  document.querySelector('.file-view')!.replaceChildren();
  window.history.replaceState(null, '', originalUrl);
});

afterAll(() => document.body.replaceChildren());

test.each(['source', 'blame'])('copies selected lines in %s view', (view) => {
  document.querySelector('.file-view')!.innerHTML = `<table class="code-view"><tbody>
    <tr><td class="lines-num"><span id="L1" data-line-number="1"></span></td><td><code class="code-inner">first\n</code></td></tr>
    <tr><td class="lines-num"><span id="L2" data-line-number="2"></span></td><td><code class="code-inner">\n</code></td></tr>
    <tr><td class="lines-num"><span id="L3" data-line-number="3"></span></td><td><code class="code-inner"><span>  last  </span>\n</code></td></tr>
  </tbody></table>
  <div class="code-line-menu">
    <button class="copy-lines" data-text-lines="Copy Lines">Copy Line</button>
    <button class="copy-line-permalink" data-url="/file?display=source#L1">Copy Permalink</button>
    ${view === 'source' ? '<a class="view_git_blame" href="/blame">View Git Blame</a>' : ''}
    <a class="ref-in-new-issue" data-url-issue-new="/issues/new" data-url-param-body-link="/file">Reference in new issue</a>
  </div>`;

  const menu = () => document.querySelector('.code-line-button')!._tippy.props.content as Element;
  document.querySelector<HTMLElement>('#L3')!.click();
  if (view === 'source') {
    expect(menu().querySelector('.copy-lines')!.textContent).toBe('Copy Line');
    expect(menu().querySelector('.copy-lines')!.getAttribute('data-clipboard-text')).toBe('  last  ');
  }

  document.querySelector('#L1')!.dispatchEvent(new MouseEvent('click', {bubbles: true, shiftKey: true}));
  expect(menu().querySelector('.copy-lines')!.getAttribute('data-clipboard-text')).toBe('first\n\n  last  ');
  if (view === 'source') {
    expect(window.location.hash).toBe('#L1-L3');
    expect(menu().querySelector('.copy-lines')!.textContent).toBe('Copy Lines');
    expect(menu().querySelector('.copy-line-permalink')!.getAttribute('data-clipboard-text')).toBe(`${window.location.origin}/file?display=source#L1-L3`);
    expect(menu().querySelector('.view_git_blame')!.getAttribute('href')).toBe('/blame#L1-L3');
    expect(menu().querySelector('.ref-in-new-issue')!.getAttribute('href')).toBe('/issues/new?body=%2Ffile%23L1-L3');
  } else {
    expect(menu().querySelector('.view_git_blame')).toBeNull();
  }
});
