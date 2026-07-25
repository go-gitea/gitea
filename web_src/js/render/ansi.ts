import {trimUrlPunctuation, urlRawRegex} from '../utils/url.ts';
import {htmlEscape} from '../utils/html.ts';

// A minimal ANSI renderer for action logs, covering only what the log viewer needs: SGR colors and
// text attributes, with the 256-color palette and truecolor. It is intentionally line-oriented, so
// an escape sequence is never carried across a line boundary. Rendering is a pure function of the
// line and the incoming AnsiStyle, so callers hold the style themselves and nothing is shared.

const replacements: Array<[RegExp, string]> = [
  [/\x1b\[\d+[A-H]/g, ''], // Move cursor, treat them as no-op
  [/\x1b\[\d?[JK]/g, '\r'], // Erase display/line, treat them as a Carriage Return
];

// In order: a CSI "\x1b[params final", an OSC 8 hyperlink, any other OSC, or "\x1b" plus one byte.
// Only SGR (a CSI with final "m") and OSC 8 are rendered, the others are recognised purely so they
// can be dropped instead of showing up as text. The OSC 8 alternative has to precede the general
// OSC one to win, and the last alternative excludes "[" and "]" so it cannot swallow an introducer.
const escapeSequence = /\x1b\[(?<params>[0-9;:?<=>]*)[\x20-\x2f]*(?<final>[\x40-\x7e])|\x1b\]8;[^;]*;(?<url>[^\x07\x1b]*)(?:\x07|\x1b\\)(?<text>[\s\S]*?)\x1b\]8;;(?:\x07|\x1b\\)|\x1b\][\s\S]*?(?:\x07|\x1b\\)|\x1b[\x20-\x5a\x5c\x5e-\x7e]/g;
// a hyperlink target has to be a web url, anything else is rendered as plain text instead
const hyperlinkUrl = /^https?:\/\//i;

type AnsiColor = {
  rgb: string, // pre-joined "r,g,b", it is only ever used to build a css color
  className: string, // empty for 256-color and truecolor, which have no css class and use a style
};

export type AnsiStyle = {
  fg: AnsiColor | null,
  bg: AnsiColor | null,
  bold: boolean,
  faint: boolean,
  italic: boolean,
  underline: boolean,
};

export const ansiStyleInitial: AnsiStyle = Object.freeze({fg: null, bg: null, bold: false, faint: false, italic: false, underline: false});

function isAnsiStyleInitial(style: AnsiStyle): boolean {
  return !style.fg && !style.bg && !style.bold && !style.faint && !style.italic && !style.underline;
}

// The 256-color palette: 0-7 normal, 8-15 bright, 16-231 a 6x6x6 rgb cube, 232-255 grayscale.
// Only the first 16 have css classes, see the "ansi-*" rules in the actions log styles.
const palette256: AnsiColor[] = (() => {
  const names = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
  const normal = ['0,0,0', '187,0,0', '0,187,0', '187,187,0', '0,0,187', '187,0,187', '0,187,187', '255,255,255'];
  const bright = ['85,85,85', '255,85,85', '0,255,0', '255,255,85', '85,85,255', '255,85,255', '85,255,255', '255,255,255'];
  const palette: AnsiColor[] = [
    ...normal.map((rgb, idx) => ({rgb, className: `ansi-${names[idx]}`})),
    ...bright.map((rgb, idx) => ({rgb, className: `ansi-bright-${names[idx]}`})),
  ];
  const levels = [0, 95, 135, 175, 215, 255];
  for (const r of levels) {
    for (const g of levels) {
      for (const b of levels) {
        palette.push({rgb: `${r},${g},${b}`, className: ''});
      }
    }
  }
  for (let grey = 8; grey <= 238; grey += 10) {
    palette.push({rgb: `${grey},${grey},${grey}`, className: ''});
  }
  return palette;
})();

function applySgr(style: AnsiStyle, params: string): AnsiStyle {
  const next = {...style};
  const codes = params.split(';');
  for (let idx = 0; idx < codes.length; idx++) {
    const code = parseInt(codes[idx], 10);
    if (isNaN(code) || code === 0) {
      next.fg = next.bg = null;
      next.bold = next.faint = next.italic = next.underline = false;
    } else if (code === 1) next.bold = true;
    else if (code === 2) next.faint = true;
    else if (code === 3) next.italic = true;
    else if (code === 4) next.underline = true;
    else if (code === 21) next.bold = false;
    else if (code === 22) next.bold = next.faint = false;
    else if (code === 23) next.italic = false;
    else if (code === 24) next.underline = false;
    else if (code === 39) next.fg = null;
    else if (code === 49) next.bg = null;
    else if (code >= 30 && code < 38) next.fg = palette256[code - 30];
    else if (code >= 40 && code < 48) next.bg = palette256[code - 40];
    else if (code >= 90 && code < 98) next.fg = palette256[code - 82]; // 8 + code - 90
    else if (code >= 100 && code < 108) next.bg = palette256[code - 92]; // 8 + code - 100
    else if ((code === 38 || code === 48) && idx + 1 < codes.length) {
      // extended color: "5;<index>" selects from the 256-color palette, "2;<r>;<g>;<b>" is truecolor.
      // A spec that runs off the end of the parameters consumes nothing beyond the mode, so the
      // remaining parameters stay available and are read as ordinary codes.
      const mode = codes[++idx];
      let color: AnsiColor | null = null;
      if (mode === '5' && idx + 1 < codes.length) {
        const paletteIndex = parseInt(codes[++idx], 10);
        if (paletteIndex >= 0 && paletteIndex <= 255) color = palette256[paletteIndex];
      } else if (mode === '2' && idx + 3 < codes.length) {
        const isByte = (value: number) => value >= 0 && value <= 255;
        const r = parseInt(codes[++idx], 10);
        const g = parseInt(codes[++idx], 10);
        const b = parseInt(codes[++idx], 10);
        if (isByte(r) && isByte(g) && isByte(b)) color = {rgb: `${r},${g},${b}`, className: ''};
      }
      if (color && code === 38) next.fg = color;
      else if (color) next.bg = color;
    }
  }
  return next;
}

function renderText(text: string, style: AnsiStyle): string {
  if (text === '') return '';
  const escaped = htmlEscape(text);
  if (isAnsiStyleInitial(style)) return escaped;

  const styles: string[] = [];
  const classes: string[] = [];
  if (style.bold) styles.push('font-weight:bold');
  if (style.faint) styles.push('opacity:0.7');
  if (style.italic) styles.push('font-style:italic');
  if (style.underline) styles.push('text-decoration:underline');
  if (style.fg) {
    if (style.fg.className) classes.push(`${style.fg.className}-fg`);
    else styles.push(`color:rgb(${style.fg.rgb})`);
  }
  if (style.bg) {
    if (style.bg.className) classes.push(`${style.bg.className}-bg`);
    else styles.push(`background-color:rgb(${style.bg.rgb})`);
  }
  const styleAttr = styles.length ? ` style="${styles.join(';')}"` : '';
  const classAttr = classes.length ? ` class="${classes.join(' ')}"` : '';
  return `<span${styleAttr}${classAttr}>${escaped}</span>`;
}

function renderPart(part: string, style: AnsiStyle): {html: string, style: AnsiStyle} {
  if (!part.includes('\x1b')) return {html: renderText(part, style), style};

  let html = '';
  let pos = 0;
  escapeSequence.lastIndex = 0; // the regex is reused, so its match position must be reset
  for (let match = escapeSequence.exec(part); match; match = escapeSequence.exec(part)) {
    if (match.index > pos) html += renderText(part.slice(pos, match.index), style);
    pos = match.index + match[0].length;
    const {params, final, url, text} = match.groups!;
    if (final === 'm') {
      style = applySgr(style, params);
    } else if (url !== undefined) {
      const label = renderText(text, style);
      html += hyperlinkUrl.test(url) ? `<a href="${htmlEscape(url)}" target="_blank">${label}</a>` : label;
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

/** Renders the lines of one log stream, carrying the ANSI style from each line into the next, so an
 * unterminated color keeps applying like it would in a terminal. Holds nothing but that style, and
 * each stream owns its own instance, so no state is shared between steps, jobs or runs.
 */
export class AnsiLineRenderer {
  private style: AnsiStyle = ansiStyleInitial;

  renderInto(el: HTMLElement, line: string): void {
    this.style = renderAnsiInto(el, line, this.style);
  }
}

/** Render one log line into el, returning the style that the next line should start from. */
export function renderAnsiInto(el: HTMLElement, line: string, style: AnsiStyle = ansiStyleInitial): AnsiStyle {
  if (line.endsWith('\r\n')) {
    line = line.substring(0, line.length - 2);
  } else if (line.endsWith('\n')) {
    line = line.substring(0, line.length - 1);
  }

  // fast path: a plain line inheriting no style renders as text, skipping the parser entirely
  if (!line.includes('\x1b') && !line.includes('\r') && isAnsiStyleInitial(style)) {
    el.textContent = line;
    if (line.includes('://')) renderAnsiPostProcessNode(el);
    return style;
  }

  if (line.includes('\x1b')) {
    for (const [regex, replacement] of replacements) {
      line = line.replace(regex, replacement);
    }
  }

  let html: string;
  if (!line.includes('\r')) {
    ({html, style} = renderPart(line, style));
  } else {
    // handle "\rReading...1%\rReading...5%\rReading...100%",
    // convert it into a multiple-line string: "Reading...1%\nReading...5%\nReading...100%"
    const lines: Array<string> = [];
    for (const part of line.split('\r')) {
      if (part === '') continue;
      const rendered = renderPart(part, style);
      style = rendered.style;
      if (rendered.html !== '') {
        lines.push(rendered.html);
      }
    }
    // the log message element is with "white-space: break-spaces;", so use "\n" to break lines
    html = lines.join('\n');
  }

  el.innerHTML = html;
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
