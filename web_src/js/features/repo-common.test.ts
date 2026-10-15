import {initRepoCloneCodespaceTabs, sanitizeRepoName, substituteRepoOpenWithUrl} from './repo-common.ts';

test('substituteRepoOpenWithUrl', () => {
  // For example: "x-github-client://openRepo/https://github.com/go-gitea/gitea"
  expect(substituteRepoOpenWithUrl('proto://a/{url}', 'https://gitea')).toEqual('proto://a/https://gitea');
  expect(substituteRepoOpenWithUrl('proto://a?link={url}', 'https://gitea')).toEqual('proto://a?link=https%3A%2F%2Fgitea');
});

test('sanitizeRepoName', () => {
  expect(sanitizeRepoName(' a b ')).toEqual('a-b');
  expect(sanitizeRepoName('a-b_c.git ')).toEqual('a-b_c');
  expect(sanitizeRepoName('/x.git/')).toEqual('-x.git-');
  expect(sanitizeRepoName('.profile')).toEqual('.profile');
  expect(sanitizeRepoName('.profile.')).toEqual('.profile');
  expect(sanitizeRepoName('.pro..file')).toEqual('.pro.file');

  expect(sanitizeRepoName('foo.rss.atom.git.wiki')).toEqual('foo');

  expect(sanitizeRepoName('.')).toEqual('');
  expect(sanitizeRepoName('..')).toEqual('');
  expect(sanitizeRepoName('-')).toEqual('');
});

test('clone panel main tabs keep selection and panels in sync', () => {
  document.body.innerHTML = `
    <div>
      <button class="repo-clone-main active" aria-selected="true"></button>
      <button class="repo-codespaces-main" aria-selected="false" tabindex="-1"></button>
      <div class="repo-clone-section"></div>
      <div class="repo-codespaces-section" hidden></div>
    </div>
  `;
  const parent = document.body.firstElementChild!;
  const cloneTab = parent.querySelector<HTMLButtonElement>('.repo-clone-main')!;
  const codespacesTab = parent.querySelector<HTMLButtonElement>('.repo-codespaces-main')!;
  const cloneSection = parent.querySelector<HTMLElement>('.repo-clone-section')!;
  const codespacesSection = parent.querySelector<HTMLElement>('.repo-codespaces-section')!;

  initRepoCloneCodespaceTabs(parent);
  codespacesTab.click();
  expect(cloneTab.getAttribute('aria-selected')).toEqual('false');
  expect(codespacesTab.getAttribute('aria-selected')).toEqual('true');
  expect(cloneTab.tabIndex).toEqual(-1);
  expect(codespacesTab.tabIndex).toEqual(0);
  expect(cloneSection.hidden).toEqual(true);
  expect(codespacesSection.hidden).toEqual(false);

  codespacesTab.dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowLeft', bubbles: true}));
  expect(cloneTab.getAttribute('aria-selected')).toEqual('true');
  expect(codespacesTab.getAttribute('aria-selected')).toEqual('false');
  expect(cloneSection.hidden).toEqual(false);
  expect(codespacesSection.hidden).toEqual(true);
});
