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
// what htmlEscape replaces, checked first because most log text contains none of it
const needsHtmlEscape = /["&'<>]/;
// "4:1" to "4:5" select an underline style, "4:0" turns it off. Indexed by number, so a non-numeric
// sub-parameter cannot reach an inherited property.
const underlineStyles = ['', 'solid', 'double', 'wavy', 'dotted', 'dashed'];

// A palette entry is either the name of a css class for one of the 16 named colors, which themes
// restyle, or a literal "#rrggbb" for the 256-color cube and truecolor, which they must not.
type AnsiColor = string;

const isThemed = (color: AnsiColor) => color[0] !== '#';

type AnsiStyle = {
  fg: AnsiColor | null,
  bg: AnsiColor | null,
  underlineColor: AnsiColor | null,
  underline: string, // '' when off, otherwise the css text-decoration-style
  bold: boolean,
  faint: boolean,
  italic: boolean,
  blink: boolean,
  strikethrough: boolean,
  overline: boolean,
  inverse: boolean,
  conceal: boolean,
};

const ansiStyleInitial: AnsiStyle = Object.freeze({
  fg: null, bg: null, underlineColor: null, underline: '',
  bold: false, faint: false, italic: false, blink: false,
  strikethrough: false, overline: false, inverse: false, conceal: false,
});

function isAnsiStyleInitial(style: AnsiStyle): boolean {
  return !style.fg && !style.bg && !style.underline && !style.bold && !style.faint &&
    !style.italic && !style.blink && !style.strikethrough && !style.overline && !style.inverse && !style.conceal;
}

// 0-7 normal, 8-15 bright, 16-231 a 6x6x6 rgb cube, 232-255 grayscale
const colorNames = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
const cubeLevels = [0, 95, 135, 175, 215, 255];
const palette256: AnsiColor[] = [
  ...['ansi-', 'ansi-bright-'].flatMap((prefix) => colorNames.map((name) => `${prefix}${name}`)),
  ...cubeLevels.flatMap((r) => cubeLevels.flatMap((g) => cubeLevels.map((b) => rgbHex(r, g, b)))),
  ...Array.from({length: 24}, (_value, idx) => rgbHex(8 + idx * 10, 8 + idx * 10, 8 + idx * 10)),
];

function rgbHex(r: number, g: number, b: number): string {
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`;
}

// the codes that only set fields, mapped to the fields they set
const sgrFields: Record<number, Partial<AnsiStyle>> = {
  1: {bold: true}, 2: {faint: true}, 3: {italic: true}, 5: {blink: true}, 7: {inverse: true},
  8: {conceal: true}, 9: {strikethrough: true}, 21: {bold: false}, 22: {bold: false, faint: false},
  23: {italic: false}, 24: {underline: ''}, 25: {blink: false}, 27: {inverse: false},
  28: {conceal: false}, 29: {strikethrough: false}, 39: {fg: null}, 49: {bg: null},
  53: {overline: true}, 55: {overline: false}, 59: {underlineColor: null},
};

function applySgr(style: AnsiStyle, params: string): AnsiStyle {
  if (params === '' || params === '0') return ansiStyleInitial; // the most common sequence by far
  const next = {...style};
  const codes = params.split(';');
  for (let idx = 0; idx < codes.length; idx++) {
    const code = parseInt(codes[idx], 10); // parseInt stops at a ":" sub-parameter on its own
    if (isNaN(code) || code === 0) {
      Object.assign(next, ansiStyleInitial);
    } else if (sgrFields[code]) Object.assign(next, sgrFields[code]);
    else if (code === 4) {
      // a colon sub-parameter selects the style, as in "4:3" for a curly underline and "4:0" for off
      const colon = codes[idx].indexOf(':');
      next.underline = colon === -1 ? 'solid' : underlineStyles[Number(codes[idx].slice(colon + 1))] ?? 'solid';
    } else if (code >= 30 && code < 38) next.fg = palette256[code - 30];
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
  let html = needsHtmlEscape.test(text) ? htmlEscape(text) : text;
  if (style.faint) html = `<span class="ansi-faint">${html}</span>`;

  const styles: string[] = [];
  const classes: string[] = [];
  if (style.bold) classes.push('ansi-bold');
  if (style.italic) classes.push('ansi-italic');
  if (style.blink) classes.push('ansi-blink');
  if (style.conceal) classes.push('ansi-conceal');
  if (style.underline || style.strikethrough || style.overline) {
    if (style.underline) classes.push('ansi-underline');
    if (style.strikethrough) classes.push('ansi-line-through');
    if (style.overline) classes.push('ansi-overline');
    if (style.underline && style.underline !== 'solid') classes.push(`ansi-${style.underline}`);
    // a slot with no class of its own still has to resolve a themed color to the theme's value
    const decorationColor = style.underlineColor && (isThemed(style.underlineColor) ? `var(--color-${style.underlineColor})` : style.underlineColor);
    if (decorationColor) styles.push(`text-decoration-color:${decorationColor}`);
  }

  // inverse swaps foreground and background, including the terminal defaults when either is unset
  const applyColor = (color: AnsiColor | null, slot: string, property: string) => {
    if (!color) {
      if (style.inverse) classes.push(`ansi-inverse-${slot}`);
    } else if (isThemed(color)) {
      classes.push(`${color}-${slot}`);
    } else {
      styles.push(`${property}:${color}`);
    }
  };
  // conceal emits no foreground at all, so an inline color can never outrank the concealing class
  if (!style.conceal) applyColor(style.inverse ? style.bg : style.fg, 'fg', 'color');
  applyColor(style.inverse ? style.fg : style.bg, 'bg', 'background-color');

  if (!classes.length && !styles.length) return html; // nothing left to apply, faint stands alone
  return `<span${classes.length ? ` class="${classes.join(' ')}"` : ''}${styles.length ? ` style="${styles.join(';')}"` : ''}>${html}</span>`;
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
  // any "\x1b" left did not form a complete sequence, so it is cut off by the end of the line and
  // can never be completed: drop it along with the rest of the line
  const cutOff = part.indexOf('\x1b', pos);
  html += renderText(part.slice(pos, cutOff === -1 ? part.length : cutOff), style);
  return {html, style};
}

/** A minimal ANSI renderer for action logs: SGR attributes and colors, the 256-color palette and
 * truecolor. Renders one log stream, carrying the style between its lines the way a terminal does,
 * while never carrying an escape sequence across a line. Each stream owns an instance.
 */
export class AnsiLineRenderer {
  private style: AnsiStyle = ansiStyleInitial;

  renderInto(el: HTMLElement, line: string): void {
    if (line.endsWith('\n')) line = line.slice(0, line.endsWith('\r\n') ? -2 : -1);

    // fast path: a plain line inheriting no style renders as text, skipping the parser entirely
    const hasEscape = line.includes('\x1b');
    if (isAnsiStyleInitial(this.style) && !hasEscape && !line.includes('\r')) {
      el.textContent = line;
      if (line.includes('://')) renderAnsiPostProcessNode(el);
      return;
    }

    if (hasEscape) line = line.replace(eraseInLine, '\r');

    // "\rReading...1%\rReading...5%\rReading...100%" becomes one rendered part per update, joined by
    // "\n" because the log message element is styled "white-space: break-spaces"
    const parts: Array<string> = [];
    for (const part of line.split('\r')) {
      if (part === '') continue;
      const rendered = renderPart(part, this.style);
      this.style = rendered.style;
      if (rendered.html !== '') parts.push(rendered.html);
    }

    el.innerHTML = parts.join('\n');
    // at the moment, only need to do post-process when there are potential URL links
    if (line.includes('://')) renderAnsiPostProcessNode(el);
  }
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
