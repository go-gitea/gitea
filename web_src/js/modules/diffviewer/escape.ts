import {html} from '../../utils/html.ts';

export type EscapeTables = {invisible: Set<number>; ambiguous: Map<number, number>};

// a replacement of characters at [start, end) of a line, with the line's status like charset.EscapeStatus
export type Escape = {start: number; end: number; html: string};
export type LineEscapes = {escapes: Escape[]; hasAmbiguous: boolean; hasInvisible: boolean};

type EscapeData = {invisible: number[]; ambiguous: Array<{locale: string; pairs: number[]}>};

const runeBasic = 0;
const runeBroken = 1;
const runeNonASCII = 2;
const runeAmbiguous = 3;
const runeInvisible = 4;
const runeControl = 5;
const longSentenceDetectionLimit = 20;

let tablesPromise: Promise<EscapeTables> | null = null;

// like AmbiguousTablesForLocale in modules/charset/ambiguous.go, the locale's table comes before the common one
function ambiguousTable(data: EscapeData, lang: string): Map<number, number> {
  const tables = new Map(data.ambiguous.map((table) => [table.locale, table.pairs]));
  let key = lang;
  let pairs: number[] | undefined;
  while (key && !pairs) {
    pairs = tables.get(key);
    const idx = Math.max(key.lastIndexOf('-'), key.lastIndexOf('_'));
    key = idx < 0 ? '' : key.substring(0, idx);
  }
  if (!pairs && (lang === 'zh-CN' || lang === 'zh_CN')) pairs = tables.get('zh-hans');
  if (!pairs && lang.startsWith('zh')) pairs = tables.get('zh-hant');
  pairs ??= tables.get('_default');
  const result = new Map<number, number>();
  for (const table of [pairs ?? [], tables.get('_common') ?? []]) {
    for (let i = 0; i < table.length; i += 2) {
      if (!result.has(table[i])) result.set(table[i], table[i + 1]);
    }
  }
  return result;
}

export function loadEscapeTables(lang: string): Promise<EscapeTables> {
  tablesPromise ??= (async () => {
    const {default: data} = await import('./unicode-escape.json') as {default: EscapeData};
    return {invisible: new Set(data.invisible), ambiguous: ambiguousTable(data, lang)};
  })();
  return tablesPromise;
}

// only lines with other characters than printable ASCII can need escaping
function mayNeedEscape(text: string): boolean {
  return /[^\t\x20-\x7e]/.test(text);
}

const runeHex = (cp: number) => `[U+${cp.toString(16).toUpperCase().padStart(4, '0')}]`;

// a port of the escape stream of modules/charset/escape_stream.go for a line of text
export function detectEscapes(text: string, tables: EscapeTables, ambiguousTitle: (char: string, confusable: string) => string): LineEscapes | null {
  if (!mayNeedEscape(text)) return null;
  const runes: Array<{cp: number; start: number; end: number; type: number; needEscape: boolean; confusable: number}> = [];
  for (let pos = 0; pos < text.length;) {
    const cp = text.codePointAt(pos)!;
    const size = cp > 0xFFFF ? 2 : 1;
    let type = runeBasic;
    let needEscape = false;
    let confusable = 0;
    if (cp === 0xFFFD) {
      type = runeBroken;
      needEscape = true;
    } else if (cp === 0x20 || cp === 0x09 || cp === 0x0A) {
      type = runeBasic;
    } else if (cp < 0x20 || cp === 0x7F) {
      type = runeControl;
      needEscape = true;
    } else if (tables.invisible.has(cp)) {
      type = runeInvisible;
    } else if (tables.ambiguous.has(cp)) {
      type = runeAmbiguous;
      confusable = tables.ambiguous.get(cp)!;
    } else if (cp >= 0x80) {
      type = runeNonASCII;
    }
    runes.push({cp, start: pos, end: pos + size, type, needEscape, confusable});
    pos += size;
  }

  const isNonASCII = (idx: number) => runes[idx]?.type === runeNonASCII || runes[idx]?.type === runeAmbiguous;
  // many non-ASCII characters around, it seems to be a sentence, escaping in it would be too noisy
  const possibleLongSentence = (pos: number) => {
    let countBasic = 0;
    let countNonASCII = 0;
    for (let i = Math.max(pos - longSentenceDetectionLimit, 0); i < Math.min(pos + longSentenceDetectionLimit, runes.length); i++) {
      if (runes[i].type === runeBasic && runes[i].cp !== 0x20) countBasic++;
      if (isNonASCII(i)) countNonASCII++;
    }
    const countChar = countBasic + countNonASCII;
    return countChar !== 0 && countNonASCII * 100 / countChar >= 50;
  };

  const result: LineEscapes = {escapes: [], hasAmbiguous: false, hasInvisible: false};
  for (const [idx, rune] of runes.entries()) {
    if ((rune.type === runeInvisible || rune.type === runeAmbiguous) && !isNonASCII(idx - 1) && !isNonASCII(idx + 1)) {
      rune.needEscape = runes.length < longSentenceDetectionLimit || !possibleLongSentence(idx);
    }
    if (!rune.needEscape) continue;
    const char = String.fromCodePoint(rune.cp);
    let escapedHtml: string;
    if (rune.type === runeBroken) {
      escapedHtml = html`<span class="broken-code-point">�</span>`;
    } else if (rune.type === runeAmbiguous) {
      result.hasAmbiguous = true;
      const confusable = String.fromCodePoint(rune.confusable);
      const title = ambiguousTitle(`${char} ${runeHex(rune.cp)}`, `${confusable} ${runeHex(rune.confusable)}`);
      escapedHtml = html`<span class="ambiguous-code-point" data-tooltip-content="${title}"><span class="char">${char}</span></span>`;
    } else if (rune.type === runeInvisible) {
      result.hasInvisible = true;
      escapedHtml = html`<span class="escaped-code-point" data-escaped="${runeHex(rune.cp)}"><span class="char">${char}</span></span>`;
    } else {
      let display = runeHex(rune.cp);
      if (rune.cp <= 0x1F) display = String.fromCodePoint(0x2400 + rune.cp);
      else if (rune.cp === 0x7F) display = '␡';
      escapedHtml = html`<span class="broken-code-point" data-escaped="${display}"><span class="char">${char}</span></span>`;
    }
    result.escapes.push({start: rune.start, end: rune.end, html: escapedHtml});
  }
  return result.escapes.length ? result : null;
}
