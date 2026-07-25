import {trimUrlPunctuation, urlRawRegex} from '../utils/url.ts';
import {htmlEscape} from '../utils/html.ts';

// erase display/line, treated as a carriage return so the line is overwritten
const eraseInLine = /\x1b\[\d?[JK]/g;
// A CSI, an OSC 8 hyperlink, any other OSC, then "\x1b" plus one byte. Only SGR (a CSI ending in
// "m") and OSC 8 render, the rest are matched so they can be dropped rather than shown as text.
// Groups are numbered because named ones cost a groups object per match, including dropped ones.
const escapeSequence = /\x1b\[([0-9;:?<=>]*)[\x20-\x2f]*([\x40-\x7e])|\x1b\]8;[^;]*;([^\x07\x1b]*)(?:\x07|\x1b\\)([\s\S]*?)\x1b\]8;;(?:\x07|\x1b\\)|\x1b\][\s\S]*?(?:\x07|\x1b\\)|\x1b[\x20-\x5a\x5c\x5e-\x7e]/g;
// a hyperlink target has to be a web url, anything else renders as plain text instead
const hyperlinkUrl = /^https?:\/\//i;
// "4:1" to "4:5" select an underline style, "4:0" turns it off. Indexed by number, so a non-numeric
// sub-parameter cannot reach an inherited property.
const underlineStyles = ['', 'solid', 'double', 'wavy', 'dotted', 'dashed'];

// A palette entry is either the name of a css class for one of the 16 named colors, which themes
// restyle, or a literal "#rrggbb" for the 256-color cube and truecolor, which they must not.
type AnsiColor = string;

const isThemed = (color: AnsiColor) => color[0] !== '#';
// for the slots that have no class, a themed color still has to resolve to the theme's value
const cssColor = (color: AnsiColor) => isThemed(color) ? `var(--color-${color})` : color;

type AnsiStyle = {
  fg: AnsiColor | null,
  bg: AnsiColor | null,
  underlineColor: AnsiColor | null,
  underline: string, // '' when off, otherwise the css text-decoration-style
  bold: boolean,
  faint: boolean,
  italic: boolean,
  strikethrough: boolean,
  overline: boolean,
  inverse: boolean,
  conceal: boolean,
};

const ansiStyleInitial: AnsiStyle = Object.freeze({
  fg: null, bg: null, underlineColor: null, underline: '',
  bold: false, faint: false, italic: false,
  strikethrough: false, overline: false, inverse: false, conceal: false,
});

function isAnsiStyleInitial(style: AnsiStyle): boolean {
  return !style.fg && !style.bg && !style.underline && !style.bold && !style.faint &&
    !style.italic && !style.strikethrough && !style.overline && !style.inverse && !style.conceal;
}

// 0-7 normal, 8-15 bright, 16-231 a 6x6x6 rgb cube, 232-255 grayscale
const colorNames = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
const cubeLevels = [0, 95, 135, 175, 215, 255];
const palette256: AnsiColor[] = [
  ...colorNames.map((name) => `ansi-${name}`),
  ...colorNames.map((name) => `ansi-bright-${name}`),
  ...cubeLevels.flatMap((r) => cubeLevels.flatMap((g) => cubeLevels.map((b) => rgbHex(r, g, b)))),
  ...Array.from({length: 24}, (_value, idx) => rgbHex(8 + idx * 10, 8 + idx * 10, 8 + idx * 10)),
];

function rgbHex(r: number, g: number, b: number): string {
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`;
}

function applySgr(style: AnsiStyle, params: string): AnsiStyle {
  const next = {...style};
  const codes = params.split(';');
  for (let idx = 0; idx < codes.length; idx++) {
    // a parameter may carry colon separated sub-parameters, as in "4:3" for a curly underline
    const [param, subParam] = codes[idx].split(':');
    const code = parseInt(param, 10);
    if (isNaN(code) || code === 0) {
      Object.assign(next, ansiStyleInitial);
    } else if (code === 1) next.bold = true;
    else if (code === 2) next.faint = true;
    else if (code === 3) next.italic = true;
    else if (code === 4) next.underline = underlineStyles[Number(subParam)] ?? 'solid';
    else if (code === 7) next.inverse = true;
    else if (code === 8) next.conceal = true;
    else if (code === 9) next.strikethrough = true;
    else if (code === 21) next.bold = false;
    else if (code === 22) next.bold = next.faint = false;
    else if (code === 23) next.italic = false;
    else if (code === 24) next.underline = '';
    else if (code === 27) next.inverse = false;
    else if (code === 28) next.conceal = false;
    else if (code === 29) next.strikethrough = false;
    else if (code === 53) next.overline = true;
    else if (code === 55) next.overline = false;
    else if (code === 59) next.underlineColor = null;
    else if (code === 39) next.fg = null;
    else if (code === 49) next.bg = null;
    else if (code >= 30 && code < 38) next.fg = palette256[code - 30];
    else if (code >= 40 && code < 48) next.bg = palette256[code - 40];
    else if (code >= 90 && code < 98) next.fg = palette256[code - 82]; // 8 + code - 90
    else if (code >= 100 && code < 108) next.bg = palette256[code - 92]; // 8 + code - 100
    else if ((code === 38 || code === 48 || code === 58) && idx + 1 < codes.length) {
      // "5;<index>" picks from the palette, "2;<r>;<g>;<b>" is truecolor, 58 colors the underline.
      // A spec running off the end consumes only the mode, leaving the rest to be read as codes.
      const mode = codes[++idx];
      let color: AnsiColor | null = null;
      if (mode === '5' && idx + 1 < codes.length) {
        const paletteIndex = parseInt(codes[++idx], 10);
        if (paletteIndex >= 0 && paletteIndex <= 255) color = palette256[paletteIndex];
      } else if (mode === '2' && idx + 3 < codes.length) {
        const r = parseInt(codes[++idx], 10);
        const g = parseInt(codes[++idx], 10);
        const b = parseInt(codes[++idx], 10);
        if (Math.min(r, g, b) >= 0 && Math.max(r, g, b) <= 255) color = rgbHex(r, g, b);
      }
      if (color) next[code === 38 ? 'fg' : code === 48 ? 'bg' : 'underlineColor'] = color;
    }
  }
  return next;
}

/** Wraps one run of text in the markup for the style in effect. Everything is a class except the
 * colors a theme must not touch. "faint" nests, so its translucent color mixes with the outer one.
 */
function renderText(text: string, style: AnsiStyle): string {
  if (text === '') return '';
  let html = htmlEscape(text);
  if (isAnsiStyleInitial(style)) return html;
  if (style.faint) html = `<span class="ansi-faint">${html}</span>`;

  const styles: string[] = [];
  const classes: string[] = [];
  if (style.bold) classes.push('ansi-bold');
  if (style.italic) classes.push('ansi-italic');
  if (style.conceal) classes.push('ansi-conceal');
  if (style.underline || style.strikethrough || style.overline) {
    if (style.underline) classes.push('ansi-underline');
    if (style.strikethrough) classes.push('ansi-line-through');
    if (style.overline) classes.push('ansi-overline');
    if (style.underline && style.underline !== 'solid') classes.push(`ansi-${style.underline}`);
    if (style.underlineColor) styles.push(`text-decoration-color:${cssColor(style.underlineColor)}`);
  }

  // inverse swaps foreground and background, including the terminal defaults when either is unset
  const fg = style.inverse ? style.bg : style.fg;
  const bg = style.inverse ? style.fg : style.bg;
  if (!fg) {
    if (style.inverse) classes.push('ansi-inverse-fg');
  } else if (isThemed(fg)) {
    classes.push(`${fg}-fg`);
  } else {
    styles.push(`color:${fg}`);
  }
  if (!bg) {
    if (style.inverse) classes.push('ansi-inverse-bg');
  } else if (isThemed(bg)) {
    classes.push(`${bg}-bg`);
  } else {
    styles.push(`background-color:${bg}`);
  }

  if (!classes.length && !styles.length) return html; // nothing left to apply, faint stands alone
  const classAttr = classes.length ? ` class="${classes.join(' ')}"` : '';
  const styleAttr = styles.length ? ` style="${styles.join(';')}"` : '';
  return `<span${classAttr}${styleAttr}>${html}</span>`;
}

function renderPart(part: string, style: AnsiStyle): {html: string, style: AnsiStyle} {
  if (!part.includes('\x1b')) return {html: renderText(part, style), style};

  let html = '';
  let pos = 0;
  escapeSequence.lastIndex = 0; // the regex is reused, so its match position must be reset
  for (let match = escapeSequence.exec(part); match; match = escapeSequence.exec(part)) {
    if (match.index > pos) html += renderText(part.slice(pos, match.index), style);
    pos = match.index + match[0].length;
    const [, params, final, url, label] = match; // see the group order on escapeSequence
    if (final === 'm') {
      style = applySgr(style, params);
    } else if (url !== undefined) {
      const text = renderText(label, style);
      html += hyperlinkUrl.test(url) ? `<a href="${htmlEscape(url)}" target="_blank">${text}</a>` : text;
    }
  }
  if (pos < part.length) {
    // any "\x1b" still left did not form a complete sequence above, so it is cut off by the end of
    // the line and can never be completed: drop it along with the rest of the line
    const tail = part.slice(pos);
    const truncated = tail.indexOf('\x1b');
    html += renderText(truncated === -1 ? tail : tail.slice(0, truncated), style);
  }
  return {html, style};
}

/** A minimal ANSI renderer for action logs: SGR attributes and colors, the 256-color palette and
 * truecolor. Renders one log stream, carrying the style between its lines the way a terminal does,
 * while never carrying an escape sequence across a line. Each stream owns an instance.
 */
export class AnsiLineRenderer {
  private style: AnsiStyle = ansiStyleInitial;

  renderInto(el: HTMLElement, line: string): void {
    this.style = renderAnsiInto(el, line, this.style);
  }
}

/** Render one log line into el, returning the style that the next line should start from. */
function renderAnsiInto(el: HTMLElement, line: string, style: AnsiStyle): AnsiStyle {
  if (line.endsWith('\r\n')) {
    line = line.substring(0, line.length - 2);
  } else if (line.endsWith('\n')) {
    line = line.substring(0, line.length - 1);
  }

  // fast path: a plain line inheriting no style renders as text, skipping the parser entirely
  const hasEscape = line.includes('\x1b');
  if (isAnsiStyleInitial(style) && !hasEscape && !line.includes('\r')) {
    el.textContent = line;
    if (line.includes('://')) renderAnsiPostProcessNode(el);
    return style;
  }

  if (hasEscape) line = line.replace(eraseInLine, '\r');

  // "\rReading...1%\rReading...5%\rReading...100%" becomes one rendered part per update, joined by
  // "\n" because the log message element is styled "white-space: break-spaces"
  const parts: Array<string> = [];
  for (const part of line.split('\r')) {
    if (part === '') continue;
    const rendered = renderPart(part, style);
    style = rendered.style;
    if (rendered.html !== '') parts.push(rendered.html);
  }

  el.innerHTML = parts.join('\n');
  // at the moment, only need to do post-process when there are potential URL links
  if (line.includes('://')) renderAnsiPostProcessNode(el);
  return style;
}

function renderAnsiProcessText(node: ChildNode): ChildNode {
  const text = node.textContent!;
  const match = urlRawRegex().exec(text);
  if (!match || match.index === undefined) return node;

  const before = text.slice(0, match.index);
  const urlMatched = match[0];
  const urlTrimmed = trimUrlPunctuation(urlMatched);
  const after = text.slice(match.index + urlMatched.length - (urlMatched.length - urlTrimmed.length));

  const link = document.createElement('a');
  link.setAttribute('href', urlTrimmed);
  link.setAttribute('target', '_blank');
  link.textContent = urlTrimmed;

  const newNodes: Array<Node | string> = [];
  if (before) newNodes.push(before);
  newNodes.push(link);
  if (after) newNodes.push(after);

  node.replaceWith(...newNodes);
  return link;
}

function renderAnsiPostProcessNode(el: ChildNode) {
  for (let node = el.firstChild; node; node = node.nextSibling) {
    if (node.nodeName === 'A') continue;
    if (node.nodeType !== Node.TEXT_NODE) {
      renderAnsiPostProcessNode(node);
      continue;
    }
    node = renderAnsiProcessText(node);
  }
}
