import {userEvent} from 'vitest/browser';
import {initRepoClonePanel, sanitizeRepoName, substituteRepoOpenWithUrl} from './repo-common.ts';

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

test('clone panel switches tabs and protocols and closes when its trigger leaves the viewport', async () => {
  document.body.innerHTML = `
    <button id="clone-trigger" style="position: fixed; top: 50px; left: 50px">Code</button>
    <div class="clone-panel-popup">
      <button class="repo-clone-main active" aria-selected="true">Clone</button>
      <button class="repo-codespaces-main" aria-selected="false" tabindex="-1">Codespaces</button>
      <div class="repo-clone-section">
        <button class="repo-clone-https" data-link="https://example.com/repo.git">HTTPS</button>
        <button class="repo-clone-ssh" data-link="git@example.com:repo.git">SSH</button>
        <button class="repo-clone-cli" data-link="gt repo clone owner/repo">CLI</button>
        <input class="repo-clone-url js-clone-url">
        <div class="repo-clone-with-apps">Open in editor</div>
      </div>
      <div class="repo-codespaces-section" hidden>Codespaces</div>
    </div>
  `;
  const button = document.querySelector<HTMLButtonElement>('#clone-trigger')!;
  initRepoClonePanel(button);
  const instance = button._tippy;
  try {
    await userEvent.click(button);
    await vi.waitFor(() => expect(instance.state.isMounted).toBe(true));
    const cloneTab = instance.popper.querySelector<HTMLButtonElement>('.repo-clone-main')!;
    const codespacesTab = instance.popper.querySelector<HTMLButtonElement>('.repo-codespaces-main')!;
    const cloneSection = instance.popper.querySelector<HTMLElement>('.repo-clone-section')!;
    const codespacesSection = instance.popper.querySelector<HTMLElement>('.repo-codespaces-section')!;
    await userEvent.click(cloneTab);
    for (const showClone of [false, true]) {
      await userEvent.keyboard(showClone ? '{ArrowLeft}' : '{ArrowRight}');
      expect(cloneTab.getAttribute('aria-selected')).toBe(String(showClone));
      expect(codespacesTab.getAttribute('aria-selected')).toBe(String(!showClone));
      expect(cloneTab.tabIndex).toBe(showClone ? 0 : -1);
      expect(codespacesTab.tabIndex).toBe(showClone ? -1 : 0);
      expect(document.activeElement).toBe(showClone ? cloneTab : codespacesTab);
      expect(cloneSection.hidden).toBe(!showClone);
      expect(codespacesSection.hidden).toBe(showClone);
    }
    for (const scheme of ['cli', 'https', 'ssh']) {
      const tab = instance.popper.querySelector<HTMLButtonElement>(`.repo-clone-${scheme}`)!;
      await userEvent.click(tab);
      expect(instance.popper.querySelector<HTMLInputElement>('input')!.value).toBe(tab.getAttribute('data-link'));
      expect(cloneSection.querySelector('.active')).toBe(tab);
      expect(instance.popper.querySelector('.repo-clone-with-apps')!.classList.contains('tw-hidden')).toBe(scheme === 'cli');
    }
    button.style.top = '-100px';
    await vi.waitFor(() => expect(instance.state.isVisible).toBe(false));
    button.style.top = '50px';
    await userEvent.click(button);
    await vi.waitFor(() => expect(instance.state.isMounted).toBe(true));
    expect(instance.state.isVisible).toBe(true);
  } finally {
    instance.destroy();
    document.body.replaceChildren();
    window.localUserSettings.setString('repo-clone-protocol', '');
  }
});
