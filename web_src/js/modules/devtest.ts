import {showSuccessToast, showInfoToast, showWarningToast, showErrorToast} from './toast.ts';
import {registerGlobalInitFunc} from './observer.ts';
import {showFomanticModal} from './fomantic/modal.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {showGlobalErrorMessage} from './errors.ts';
import {AnsiLineRenderer} from '../render/ansi.ts';
import type {Intent} from '../types.ts';

function initDevtestPage() {
  const toastButtons = document.querySelectorAll('.toast-test-button');
  if (toastButtons.length) {
    const levelMap = {success: showSuccessToast, info: showInfoToast, warning: showWarningToast, error: showErrorToast};
    for (const el of toastButtons) {
      el.addEventListener('click', () => {
        const level = el.getAttribute('data-toast-level') as Intent;
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

// shows every sequence web_src/js/render/ansi.ts renders. Lines whose output does not explain
// itself are preceded by their own escaped source.
function initDevtestAnsiRender(container: HTMLElement) {
  const esc = '\x1b';
  const cells = (count: number, cell: (index: number) => string, separator = '') =>
    Array.from({length: count}, (_value, index) => cell(index)).join(separator);
  const attr = (params: string, label: string) => `${esc}[${params}m${label}${esc}[m`;
  const withSource = (line: string) => [attr('2', line.replaceAll(esc, '\\e').replaceAll('\b', '\\b').replaceAll('\r', '\\r')), line];

  const lines = [
    ...Array.from({length: 16}, (_value, row) =>
      cells(16, (col) => `${esc}[38;5;${row * 16 + col}m${String(row * 16 + col).padStart(4)}${esc}[0m`)),
    ' ',
    cells(16, (index) => `${esc}[48;5;${index}m ${String(index).padStart(3)} ${esc}[0m`),
    // truecolor, a gradient no palette index can express
    cells(77, (col) => `${esc}[48;2;${255 - col * 3};0;${col * 3}m${esc}[38;2;${col * 3};0;${255 - col * 3}m/${esc}[0m`),
    ' ',
    [cells(10, (code) => attr(String(code), `SGR ${code}`), '  '), attr('53', 'SGR 53')].join('  '),
    ' ',
    [
      cells(5, (index) => attr(`4:${index + 1}`, `SGR 4:${index + 1}`), '  '),
      attr('21', 'SGR 21'),
      `${esc}[4:3m${esc}[58;2;135;0;255mtruecolor underline${esc}[59m${esc}[4:0m`,
      `${esc}]8;;https://example.com${esc}\\${esc}[3mstyled${esc}[23m hyperlink${esc}]8;;${esc}\\`,
    ].join('  '),
    ' ',
    ...withSource('Reading... 1%\rReading... 50%\rReading... 100%'),
    ...withSource(`first${esc}[Ksecond${esc}[2Jthird`),
    ...withSource(`cursor ${esc}[3Amovement, private ${esc}[?25lCSI, ${esc}]0;title${esc}\\titles, ${esc}Pquery${esc}\\strings, truncated${esc}[38;5;`),
    ...withSource('Reading... 10%\b\b\b100%'),
    ...withSource('<script>alert(1)</script> & "quotes", and a bare url https://example.com'),
    ' ',
    `${esc}[31man unterminated color`,
    'carries into the following lines',
    `${esc}[0muntil something resets it`,
  ];

  const elConsole = createElementFromHTML(html`<div class="console tw-p-2 tw-whitespace-pre-wrap"></div>`);
  const ansi = new AnsiLineRenderer();
  for (const line of lines) {
    const el = document.createElement('div');
    ansi.renderLine(el, line);
    elConsole.append(el);
  }
  container.append(elConsole);
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
