import {initPackagesManualVars} from './packages.ts';
import {initMarkupCodeCopy} from '../markup/codecopy.ts';
import {createElementFromHTML} from '../utils/dom.ts';

test('initPackagesManualVars', () => {
  const el = createElementFromHTML<HTMLElement>(`
<div>
  <select data-code-var="distribution"><option value="debian"></option><option value="ubuntu"></option></select>
  <select data-code-var="component"><option value="stable"></option><option value="testing"></option></select>
  <div class="code-block"><code>sudo apt install gitea-<span>$distribution</span>-<span>$component</span>
</code></div>
  <div class="code-block"><code>unrelated command
</code></div>
</div>
`);
  initMarkupCodeCopy(el);
  initPackagesManualVars(el);
  const elSelects = el.querySelectorAll<HTMLSelectElement>('select');
  const elCodeBlocks = el.querySelectorAll<HTMLElement>('.code-block');
  const elCode = elCodeBlocks[0].querySelector('code')!;
  const elCopyButtons = el.querySelectorAll<HTMLButtonElement>('[data-clipboard-text]');

  const visibleCommand = () => elCode.textContent.replace(/\r?\n$/, '');
  const expectCopy = (command: string) => {
    expect(visibleCommand()).toBe(command);
    expect(elCopyButtons[0].getAttribute('data-clipboard-text')).toBe(command);
    expect(elCopyButtons[1].getAttribute('data-clipboard-text')).toBe('unrelated command');
  };

  expectCopy('sudo apt install gitea-debian-stable');

  elSelects[0].value = 'ubuntu';
  elSelects[0].dispatchEvent(new Event('change'));
  expectCopy('sudo apt install gitea-ubuntu-stable');

  elSelects[1].value = 'testing';
  elSelects[1].dispatchEvent(new Event('change'));
  expectCopy('sudo apt install gitea-ubuntu-testing');
});
