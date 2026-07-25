import {showInfoToast, showWarningToast, showErrorToast} from './toast.ts';
import type {Toast} from './toast.ts';
import {registerGlobalInitFunc} from './observer.ts';
import {showFomanticModal} from './fomantic/modal.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {showGlobalErrorMessage} from './errors.ts';
import {AnsiLineRenderer} from '../render/ansi.ts';

type LevelMap = Record<string, (message: string) => Toast | null>;

function initDevtestPage() {
  const toastButtons = document.querySelectorAll('.toast-test-button');
  if (toastButtons.length) {
    const levelMap: LevelMap = {info: showInfoToast, warning: showWarningToast, error: showErrorToast};
    for (const el of toastButtons) {
      el.addEventListener('click', () => {
        const level = el.getAttribute('data-toast-level')!;
        const message = el.getAttribute('data-toast-message')!;
        levelMap[level](message);
      });
    }
    document.querySelector('.toast-test-button-pre')!.addEventListener('click', () => {
      showErrorToast(html`<div>message <pre>pre ${'a'.repeat(200)}</pre><details><summary>summary</summary>details</details></div>`, {useHtmlBody: true});
    });
  }

  const modalButtons = document.querySelector('.modal-buttons');
  if (modalButtons) {
    for (const el of document.querySelectorAll('.ui.modal:not([data-skip-button])')) {
      const btn = createElementFromHTML(html`<button class="ui button">${el.id}</button`);
      btn.addEventListener('click', () => showFomanticModal(el));
      modalButtons.append(btn);
    }
  }

  const sampleButtons = document.querySelectorAll('#devtest-button-samples button.ui.button');
  if (sampleButtons.length) {
    const buttonStyles = document.querySelectorAll<HTMLInputElement>('input[name*="button-style"]');
    for (const elStyle of buttonStyles) {
      elStyle.addEventListener('click', () => {
        for (const btn of sampleButtons) {
          for (const el of buttonStyles) {
            if (el.value) btn.classList.toggle(el.value, el.checked);
          }
        }
      });
    }
    const buttonStates = document.querySelectorAll<HTMLInputElement>('input[name*="button-state"]');
    for (const elState of buttonStates) {
      elState.addEventListener('click', () => {
        for (const btn of sampleButtons) {
          (btn as any)[elState.value] = elState.checked;
        }
      });
    }
  }
}

// Showcase for every feature of the ANSI renderer.
function initDevtestAnsiRender(container: HTMLElement) {
  const esc = '\x1b';
  const cells = (count: number, cell: (index: number) => string) => Array.from({length: count}, (_value, index) => cell(index));
  const join = (count: number, cell: (index: number) => string) => cells(count, cell).join('');
  const attr = (params: string, label: string) => `${esc}[${params}m${label}${esc}[m`;

  const sections: Array<{title: string, lines: string[], source?: boolean}> = [
    // indexes 0-15 are the named colors a theme restyles, the rest are literal
    {title: 'colors', lines: [
      ...Array.from({length: 16}, (_value, row) =>
        join(16, (col) => `${esc}[38;5;${row * 16 + col}m${String(row * 16 + col).padStart(4)}${esc}[0m`)),
      ' ',
      join(16, (index) => `${esc}[48;5;${index}m ${String(index).padStart(3)} ${esc}[0m`),
      join(16, (index) => `${esc}[48;5;${16 + index * 13}m ${String(16 + index * 13).padStart(3)} ${esc}[0m`),
      ' ',
      join(77, (col) => { // truecolor, a gradient no palette index can express
        const r = 255 - Math.floor(col * 255 / 76);
        const b = Math.floor(col * 255 / 76);
        const g = Math.min(Math.floor(col * 510 / 76), 510 - Math.floor(col * 510 / 76));
        return `${esc}[48;2;${r};${g};${b}m${esc}[38;2;${255 - r};${255 - g};${255 - b}m${'/\\'[col % 2]}${esc}[0m`;
      }),
    ]},

    {title: 'attributes', lines: [
      [...cells(10, (code) => attr(String(code), `SGR ${code}`)), attr('53', 'SGR 53')].join('  '),
      ' ',
      [
        ...cells(5, (index) => attr(`4:${index + 1}`, `SGR 4:${index + 1}`)),
        attr('21', 'SGR 21'),
        `${esc}[4:3m${esc}[58;2;135;0;255mtruecolor underline${esc}[59m${esc}[4:0m`,
        `${esc}]8;;https://example.com${esc}\\hyperlink${esc}]8;;${esc}\\`,
      ].join('  '),
    ]},

    // rendered under its own escaped source, since what these do is not visible from the output alone
    {title: 'behavior', source: true, lines: [
      `${esc}[31man unterminated color`,
      'carries into the following lines',
      `${esc}[0muntil something resets it`,
      'Reading... 1%\rReading... 50%\rReading... 100%',
      `first${esc}[Ksecond${esc}[2Jthird`,
      `cursor movement ${esc}[3Ais dropped`,
      `private CSI ${esc}[?25lis dropped`,
      `${esc}]0;window title${esc}\\a window title is dropped`,
      `a sequence cut off by the line end is dropped${esc}[38;5;`,
      '<script>alert(1)</script> & "quotes" are escaped',
      'a bare url such as https://example.com becomes a link',
    ]},
  ];

  for (const {title, lines, source} of sections) {
    container.append(createElementFromHTML(html`<h2>${title}</h2>`));
    const elConsole = createElementFromHTML(html`<div class="console tw-p-2 tw-mb-4 tw-whitespace-pre-wrap"></div>`);
    const ansi = new AnsiLineRenderer(); // one per section, so a section cannot bleed into the next
    for (const line of lines) {
      if (source) elConsole.append(createElementFromHTML(html`<div class="tw-mt-2 tw-opacity-50">${line.replaceAll(esc, '\\e')}</div>`));
      const el = document.createElement('div');
      ansi.renderInto(el, line);
      elConsole.append(el);
    }
    container.append(elConsole);
  }
}

export function initDevtest() {
  registerGlobalInitFunc('initDevtestPage', initDevtestPage);
  registerGlobalInitFunc('initDevtestAnsiRender', initDevtestAnsiRender);
  registerGlobalInitFunc('initDevtestDetailsErrorMessage', () => {
    for (let i = 0; i < 2; i++) {
      showGlobalErrorMessage('showGlobalErrorMessage single message', 'warning');
      showGlobalErrorMessage('showGlobalErrorMessage message with details', 'error', `detail message 1\nvery lo${'o'.repeat(200)}ng line 2\nline 3`);
    }
  });
}
