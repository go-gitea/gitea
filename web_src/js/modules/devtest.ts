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

// Showcase for every feature of the ANSI renderer, generated so the palettes stay exhaustive.
function initDevtestAnsiRender(container: HTMLElement) {
  const esc = '\x1b';
  const sgr = (params: string, text: string) => `${esc}[${params}m${text}${esc}[0m`;
  const row = (count: number, cell: (index: number) => string) => Array.from({length: count}, (_value, index) => cell(index)).join('');
  const palette = (params: (index: number) => string) => Array.from({length: 16}, (_value, line) =>
    row(16, (col) => sgr(params(line * 16 + col), String(line * 16 + col).padStart(4))));

  const sections: Array<{title: string, lines: string[], source?: boolean}> = [
    {title: '16 colors, foreground over background, normal / bold / bright', lines: [
      ...['0', '1'].flatMap((attrs) => Array.from({length: 8}, (_value, fg) =>
        row(8, (bg) => sgr(`${attrs};${30 + fg};${40 + bg}`, ` ${attrs};${30 + fg};${40 + bg} `)))),
      ...Array.from({length: 8}, (_value, fg) =>
        row(8, (bg) => sgr(`${90 + fg};${100 + bg}`, ` ${90 + fg};${100 + bg} `))),
    ]},
    {title: 'attributes, in the layout of the colors.py sample from ansi_up issue 78', lines: [
      `${row(10, (code) => ` ${esc}[${code}mSGR ${String(code).padStart(2)}${esc}[m `)} ${esc}[53mSGR 53${esc}[m`,
      `${row(5, (idx) => ` ${esc}[4:${idx + 1}mSGR 4:${idx + 1}${esc}[m `)} ${esc}[21mSGR 21${esc}[m`,
      ` ${esc}[4:3m${esc}[58;2;135;0;255mtruecolor underline${esc}[59m${esc}[4:0m  ${esc}]8;;https://example.com${esc}\\hyperlink${esc}]8;;${esc}\\`,
      `${esc}[1;3;4;31mall on${esc}[22m no bold${esc}[23m no italic${esc}[24m no underline${esc}[39m default color`,
    ]},
    {title: '256 colors, foreground (0-15 use css classes, the rest inline colors)', lines: palette((index) => `38;5;${index}`)},
    {title: '256 colors, faint (SGR 2)', lines: palette((index) => `2;38;5;${index}`)},
    {title: '256 colors, background', lines: palette((index) => `48;5;${index}`)},
    {title: 'truecolor gradient', lines: [row(77, (col) => {
      const r = 255 - Math.floor(col * 255 / 76);
      const b = Math.floor(col * 255 / 76);
      const g = Math.min(Math.floor(col * 510 / 76), 510 - Math.floor(col * 510 / 76));
      return `${esc}[48;2;${r};${g};${b}m${esc}[38;2;${255 - r};${255 - g};${255 - b}m${'/\\'[col % 2]}${esc}[0m`;
    })]},
    {title: 'style carries into the following lines, like a terminal', lines: [
      `${esc}[31man unterminated color`,
      'carries into the following lines',
      `${esc}[1mand combines with attributes set later`,
      `${esc}[0muntil something resets it`,
      'back to plain',
    ]},
    {title: 'sequences that are handled but not displayed', source: true, lines: [
      'Reading... 1%\rReading... 50%\rReading... 100%',
      `first${esc}[Ksecond${esc}[2Jthird`,
      `cursor movement ${esc}[3Ais dropped`,
      `private CSI ${esc}[?25lis dropped`,
      `${esc}]0;window title${esc}\\OSC window title is dropped`,
      `${esc}]8;;https://example.com${esc}\\OSC 8 hyperlink becomes a link${esc}]8;;${esc}\\`,
      `a sequence cut off by the line end is dropped${esc}[38;5;`,
      '<script>alert(1)</script> & "quotes" are escaped',
      'urls such as https://example.com/path?a=b&c=d become links',
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
