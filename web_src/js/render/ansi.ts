import {trimUrlPunctuation, urlRawRegex} from '../utils/url.ts';
import {createElementFromAttrs} from '../utils/dom.ts';
import {colord} from 'colord';

// erase display/line, treated as a carriage return
const eraseInLine = /\x1b\[\d?[JK]/g;
// a CSI, an OSC 8 hyperlink, any other string sequence (OSC, DCS, SOS, PM, APC), then an escape with
// its intermediates. Only SGR ("m") and OSC 8 render, the rest are matched so they can be dropped.
// A string sequence ends at BEL, "\x1b\\", the 8-bit ST "\x9c", or the next escape.
const escapeSequence = /\x1b\[([0-9;:?<=>]*)[\x20-\x2f]*([\x40-\x7e])|\x1b\]8;[^;\x07\x1b\x9c]*;([^\x07\x1b\x9c]*)(?:\x07|\x1b\\|\x9c)|\x1b[\]P^_X][^\x07\x1b\x9c]*(?:\x07|\x1b\\|\x9c)?|\x1b[\x20-\x2f]*[\x30-\x5a\x5c-\x7e]/g;
const hyperlinkUrl = /^https?:\/\//i;
// a CSI marked private carries no SGR, whatever its final byte
const privateParams = /^[<=>?]/;
// characters a terminal never shows, other than tab, "\n" and "\r"
const controlChars = /[\x00-\x08\v\f\x0e-\x1f\x7f-\x9f]/g;
const hasControlChar = new RegExp(controlChars.source);
// the same less the backspace, which the column model consumes
const controlCharsNoBackspace = /[\x00-\x07\v\f\x0e-\x1f\x7f-\x9f]/g;
// a line with none of these is plain text, an escape being a control character
const needsRendering = new RegExp(`${controlChars.source}|\\r|://`);
// "4:1" to "4:5" select a style, "4:0" is off. Indexed by number, so a non-numeric one cannot
// reach an inherited property.
const underlineStyles = ['', 'solid', 'double', 'wavy', 'dotted', 'dashed'];

// a css class for the 16 named colors a theme restyles, or "#rrggbb" for the rest, which it must not
type AnsiColor = string;

const anchor = (href: string, ...children: string[]) =>
  createElementFromAttrs<HTMLAnchorElement>('a', {href, target: '_blank'}, ...children);

// appends text, turning any bare url inside it into a link
function appendText(target: ParentNode, text: string, linkify: boolean): void {
  if (!linkify || !text.includes('://')) {
    if (target.firstChild) target.append(text); else target.textContent = text; // one step into an empty target
    return;
  }
  const urls = urlRawRegex();
  let pos = 0;
  for (let match = urls.exec(text); match; match = urls.exec(text)) {
    const url = trimUrlPunctuation(match[0]);
    if (match.index > pos) target.append(text.slice(pos, match.index));
    target.append(anchor(url, url));
    urls.lastIndex = pos = match.index + url.length;
  }
  if (pos < text.length) target.append(text.slice(pos));
}

type AnsiStyle = {
  fg: AnsiColor | null, bg: AnsiColor | null, underlineColor: AnsiColor | null,
  underline: string,
  bold: boolean, faint: boolean, italic: boolean, blink: boolean,
  strikethrough: boolean, overline: boolean, inverse: boolean, conceal: boolean,
};

const ansiStyleInitial: Readonly<AnsiStyle> = {
  fg: null, bg: null, underlineColor: null, underline: '',
  bold: false, faint: false, italic: false, blink: false,
  strikethrough: false, overline: false, inverse: false, conceal: false,
};

// 0-7 normal, 8-15 bright, 16-231 a 6x6x6 rgb cube, 232-255 grayscale, the values the
// ".term-fgx*" rules hardcode for the console renderer
const colorNames = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
const cubeLevels = [0, 95, 135, 175, 215, 255];
const palette256: AnsiColor[] = [
  ...['ansi-', 'ansi-bright-'].flatMap((prefix) => colorNames.map((name) => `${prefix}${name}`)),
  ...cubeLevels.flatMap((r) => cubeLevels.flatMap((g) => cubeLevels.map((b) => colord({r, g, b}).toHex()))),
  ...Array.from({length: 24}, (_value, idx) => colord({r: 8 + idx * 10, g: 8 + idx * 10, b: 8 + idx * 10}).toHex()),
];

// 6 is the rapid blink a terminal renders no differently from the slow blink of 5
const sgrFields: Record<number, Partial<AnsiStyle>> = {
  1: {bold: true}, 2: {faint: true}, 3: {italic: true}, 5: {blink: true}, 6: {blink: true}, 7: {inverse: true},
  8: {conceal: true}, 9: {strikethrough: true}, 21: {bold: false}, 22: {bold: false, faint: false},
  23: {italic: false}, 24: {underline: ''}, 25: {blink: false}, 27: {inverse: false},
  28: {conceal: false}, 29: {strikethrough: false}, 39: {fg: null}, 49: {bg: null},
  53: {overline: true}, 55: {overline: false}, 59: {underlineColor: null},
};

function applySgr(style: Readonly<AnsiStyle>, params: string): Readonly<AnsiStyle> {
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
    else if (code === 38 || code === 48 || code === 58) {
      // "5;<index>" picks from the palette, "2;<r>;<g>;<b>" is truecolor, 58 colors the underline,
      // and ":" sub-parameters carry the same with "2" optionally preceded by a color space id
      if (codes[idx].includes(':')) {
        const sub = codes[idx].split(':');
        if (sub.length === 6 && sub[1] === '2') sub.splice(2, 1);
        codes.splice(idx, 1, ...sub);
      }
      // one running off the end consumes only the mode
      const mode = codes[++idx];
      let color: AnsiColor | null = null;
      if (mode === '5' && idx + 1 < codes.length) {
        const paletteIndex = parseInt(codes[++idx], 10);
        if (paletteIndex >= 0 && paletteIndex <= 255) color = palette256[paletteIndex];
      } else if (mode === '2' && idx + 3 < codes.length) {
        const [r, g, b] = [codes[++idx], codes[++idx], codes[++idx]].map((value) => parseInt(value, 10));
        if (Math.min(r, g, b) >= 0 && Math.max(r, g, b) <= 255) color = colord({r, g, b}).toHex();
      }
      if (color) next[code === 38 ? 'fg' : code === 48 ? 'bg' : 'underlineColor'] = color;
    }
  }
  return next;
}

type AnsiRun = {text: string, style: Readonly<AnsiStyle>};

/** Replays the backspaces over a part, so what follows a cursor step back overwrites what sits
 * there, keeping the style that wrote it rather than the one it displaced. */
function overwriteColumns(runs: AnsiRun[]): AnsiRun[] {
  const columns: string[] = [];
  const styles: Array<Readonly<AnsiStyle>> = [];
  let col = 0;
  for (const run of runs) {
    for (const char of run.text.replace(controlCharsNoBackspace, '')) { // by code point, so a pair stays whole
      if (char === '\b') col = Math.max(col - 1, 0);
      else {
        columns[col] = char;
        styles[col] = run.style;
        col++;
      }
    }
  }
  const merged: AnsiRun[] = [];
  for (let idx = 0; idx < columns.length; idx++) {
    if (idx && styles[idx] === styles[idx - 1]) merged[merged.length - 1].text += columns[idx];
    else merged.push({text: columns[idx], style: styles[idx]});
  }
  return merged;
}

function renderText(target: ParentNode, text: string, style: Readonly<AnsiStyle>, linkify = true): void {
  if (hasControlChar.test(text)) text = text.replace(controlChars, '');
  if (text === '') return;

  const classes: string[] = [];
  // the one place deciding class vs inline, a named color having a class per slot
  const colorValue = (color: AnsiColor | null, slot?: string) => {
    if (!color) {
      if (slot && style.inverse) classes.push(`ansi-inverse-${slot}`);
      return '';
    }
    if (color[0] === '#') return color; // a literal color is never themed
    if (!slot) return `var(--color-${color})`; // a decoration color has no class of its own
    classes.push(`${color}-${slot}`);
    return '';
  };
  if (style.bold) classes.push('ansi-bold');
  if (style.italic) classes.push('ansi-italic');
  if (style.blink) classes.push('ansi-blink');
  if (style.conceal) classes.push('ansi-conceal');
  if (style.underline) classes.push('ansi-underline');
  if (style.strikethrough) classes.push('ansi-line-through');
  if (style.overline) classes.push('ansi-overline');
  if (style.underline && style.underline !== 'solid') classes.push(`ansi-${style.underline}`);

  const decorated = style.underline || style.strikethrough || style.overline;
  const decorationColor = style.underlineColor && decorated ? colorValue(style.underlineColor) : '';
  // inverse swaps the two, terminal defaults included, and conceal emits no foreground at all so
  // that an inline color can never outrank the concealing class
  const color = style.conceal ? '' : colorValue(style.inverse ? style.bg : style.fg, 'fg');
  const background = colorValue(style.inverse ? style.fg : style.bg, 'bg');

  if (classes.length || decorationColor || color || background) {
    const span = document.createElement('span');
    if (classes.length) span.className = classes.join(' ');
    // each declaration on its own, so no string from a log can widen what it applies to
    if (decorationColor) span.style.textDecorationColor = decorationColor;
    if (color) span.style.color = color;
    if (background) span.style.backgroundColor = background;
    target.append(span);
    target = span;
  }
  if (style.faint) { // nested, so its translucent color mixes with the color the outer span applies
    const faint = document.createElement('span');
    faint.className = 'ansi-faint';
    target.append(faint);
    target = faint;
  }
  appendText(target, text, linkify);
}

/** Renders one log stream, carrying the style between lines the way a terminal does but never an
 * escape sequence. Each stream owns an instance. */
export class AnsiLineRenderer {
  private style: Readonly<AnsiStyle> = ansiStyleInitial;

  private renderPart(target: ParentNode, part: string): void {
    const overwriting = part.includes('\b'); // a backspace reaches back, so nothing renders until the end
    if (!overwriting && !part.includes('\x1b')) {
      renderText(target, part, this.style);
      return;
    }

    let pos = 0;
    let pending = ''; // text is held until the style changes, so an escape between runs cannot split them
    let container: ParentNode = target; // an open OSC 8 hyperlink, collecting the styled text
    const runs: AnsiRun[] = [];
    const flush = () => {
      if (pending) runs.push({text: pending, style: this.style});
      pending = '';
    };
    const emit = () => {
      // a run inside a hyperlink is not linkified again
      for (const run of overwriting ? overwriteColumns(runs) : runs) renderText(container, run.text, run.style, container === target);
      runs.length = 0;
    };

    escapeSequence.lastIndex = 0; // the regex is reused, so its match position must be reset
    for (let match = escapeSequence.exec(part); match; match = escapeSequence.exec(part)) {
      if (match.index > pos) pending += part.slice(pos, match.index);
      pos = match.index + match[0].length;
      const [, params, final, url] = match; // see the group order on escapeSequence
      if (final === 'm' && !privateParams.test(params)) {
        flush();
        this.style = applySgr(this.style, params);
      } else if (url !== undefined) { // an OSC 8, with an empty url when it closes a hyperlink
        flush();
        emit(); // a hyperlink is an element of its own, so a backspace cannot reach back across it
        // any scheme but http(s) renders as plain text, and a hyperlink never spills past the part
        const link = hyperlinkUrl.test(url) ? anchor(url) : null;
        if (link) target.append(link);
        container = link ?? target;
      }
    }
    const cutOff = part.indexOf('\x1b', pos); // a leftover escape is a sequence cut off by the line end
    pending += part.slice(pos, cutOff === -1 ? part.length : cutOff);
    flush();
    emit();
  }

  renderLine(el: HTMLElement, line: string): void {
    if (line.endsWith('\n')) line = line.slice(0, line.endsWith('\r\n') ? -2 : -1);

    // a plain line inheriting no style skips the parser entirely
    if (this.style === ansiStyleInitial && !needsRendering.test(line)) {
      el.textContent = line;
      return;
    }

    if (line.includes('\x1b')) line = line.replace(eraseInLine, '\r');

    if (el.firstChild) el.replaceChildren(); // nothing to clear for the fresh element a log line owns
    if (!line.includes('\r')) {
      this.renderPart(el, line); // no fragment, so the nodes are never built only to be moved
      return;
    }

    // one part per update, separated by "\n" for "white-space: break-spaces"
    for (const part of line.split('\r')) {
      if (!part) continue;
      const previous = el.lastChild;
      this.renderPart(el, part);
      // a part that rendered nothing needs no separator
      if (previous && previous !== el.lastChild) el.insertBefore(document.createTextNode('\n'), previous.nextSibling);
    }
  }
}
