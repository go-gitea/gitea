import type {DiffHunk, DiffLine, DiffSide} from './parse.ts';
import {html, htmlEscape, htmlRaw} from '../../utils/html.ts';
import {svgRaw} from '../../svg.ts';
import {highlightLines, type LineHighlighter, type Segment} from './highlight.ts';
import {wordDiffRanges, type CharRange} from './worddiff.ts';
import type {Escape, LineEscapes} from './escape.ts';

type RowRenderOptions = {
  addCommentLabel: string | null; // null when commenting is not possible
  escapeButtons: string[]; // the unicode escape toggle of each line in hunk order
  expanders: Map<number, string>; // the expand buttons of the gap above each hunk, the tail gap uses hunks.length
};

const typeMarkers = {same: '', add: '+', del: '-'};
const spanClose = html`</span>`;

export const unifiedColgroup = html`<colgroup><col width="50"><col width="50"><col width="10"><col width="10"><col></colgroup>`;
export const splitColgroup = html`<colgroup><col width="50"><col width="10"><col width="10"><col><col width="50"><col width="10"><col width="10"><col></colgroup>`;

export function hunksLineCount(hunks: DiffHunk[]): number {
  let count = 0;
  for (const hunk of hunks) count += hunk.lines.length + 1;
  return count;
}

// pairs the n-th line of a run of deleted lines with the n-th line of the added lines following it,
// returns the paired line index of each line in hunk order, or -1
export function pairChangedLines(hunks: DiffHunk[]): Int32Array {
  const pairs = new Int32Array(hunks.reduce((sum, hunk) => sum + hunk.lines.length, 0)).fill(-1);
  let base = 0;
  for (const {lines} of hunks) {
    let i = 0;
    while (i < lines.length) {
      if (lines[i].type !== 'del') {
        i++;
        continue;
      }
      const delStart = i;
      while (i < lines.length && lines[i].type === 'del') i++;
      const addStart = i;
      while (i < lines.length && lines[i].type === 'add') i++;
      for (let k = 0; k < Math.min(addStart - delStart, i - addStart); k++) {
        pairs[base + delStart + k] = base + addStart + k;
        pairs[base + addStart + k] = base + delStart + k;
      }
    }
    base += lines.length;
  }
  return pairs;
}

// renders highlighted segments, wrapping the parts within the ranges (sorted, not overlapping) into rangeClass,
// and replacing the characters of the escapes (sorted) by their HTML
export function segmentsToHtml(segments: Segment[], ranges: CharRange[] = [], rangeClass = '', escapes: Escape[] = []): string {
  let out = '';
  let pos = 0;
  let rangeIdx = 0;
  let escapeIdx = 0;
  const rangeOpen = html`<span class="${rangeClass}">`;
  for (const [text, classes] of segments) {
    const classOpen = classes ? html`<span class="${classes}">` : '';
    let start = 0;
    while (start < text.length) {
      const at = pos + start;
      while (rangeIdx < ranges.length && ranges[rangeIdx][1] <= at) rangeIdx++;
      while (escapeIdx < escapes.length && escapes[escapeIdx].end <= at) escapeIdx++;
      const range = ranges.at(rangeIdx);
      const inRange = Boolean(range && range[0] <= at);
      const escape = escapes.at(escapeIdx);
      let end = range ? Math.min(text.length, (inRange ? range[1] : range[0]) - pos) : text.length;
      let piece: string;
      if (escape && escape.start <= at) {
        piece = escape.html;
        end = Math.min(text.length, escape.end - pos);
        escapeIdx++;
      } else {
        if (escape) end = Math.min(end, escape.start - pos);
        piece = htmlEscape(text.slice(start, end));
      }
      const pieceHtml = classOpen ? classOpen + piece + spanClose : piece;
      out += inRange ? rangeOpen + pieceHtml + spanClose : pieceHtml;
      start = end;
    }
    pos += text.length;
  }
  return out;
}

function addButton(opts: RowRenderOptions, side: DiffSide): string {
  if (opts.addCommentLabel === null) return '';
  return html`<button type="button" class="ui primary button add-code-comment" data-side="${side}" aria-label="${opts.addCommentLabel}">${svgRaw('octicon-plus')}</button>`;
}

// the row of a hunk header, with the expand buttons of the gap above it, the tail gap has an empty header
function hunkRow(hunkIdx: number, header: string, isSplit: boolean, opts: RowRenderOptions): string {
  const cells = isSplit ?
    html`<td class="lines-num lines-num-old">${htmlRaw(opts.expanders.get(hunkIdx) ?? '')}</td><td class="lines-escape lines-escape-old"></td><td colspan="6" class="chroma lines-code lines-code-old"><code class="code-inner">${header}</code></td>` :
    html`<td colspan="2" class="lines-num">${htmlRaw(opts.expanders.get(hunkIdx) ?? '')}</td><td class="lines-escape"></td><td class="lines-type-marker"><span class="tw-font-mono" data-type-marker=""></span></td><td class="chroma lines-code blob-hunk"><code class="code-inner">${header}</code></td>`;
  return html`<tr class="tag-code" data-line-type="tag" data-hunk="${hunkIdx}">${htmlRaw(cells)}</tr>`;
}

function tailRow(hunks: DiffHunk[], isSplit: boolean, opts: RowRenderOptions): string {
  return opts.expanders.has(hunks.length) ? hunkRow(hunks.length, '', isSplit, opts) : '';
}

// lineHtml holds the content of all lines in hunk order
export function renderUnifiedRows(hunks: DiffHunk[], lineHtml: string[], opts: RowRenderOptions): string {
  let out = '';
  let idx = 0;
  for (const [hunkIdx, hunk] of hunks.entries()) {
    out += hunkRow(hunkIdx, hunk.header, false, opts);
    for (const line of hunk.lines) {
      const oldNum = line.oldNum || '';
      const newNum = line.newNum || '';
      out += html`<tr class="${line.type}-code${line.expanded ? ' line-expanded' : ''}" data-line-type="${line.type}" data-old="${oldNum}" data-new="${newNum}"><td class="lines-num lines-num-old" data-line-num="${oldNum}"></td><td class="lines-num lines-num-new" data-line-num="${newNum}"></td><td class="lines-escape">${htmlRaw(opts.escapeButtons[idx])}</td><td class="lines-type-marker"><span class="tw-font-mono" data-type-marker="${typeMarkers[line.type]}"></span></td><td class="chroma lines-code${line.type === 'del' ? ' lines-code-old' : ''}">${htmlRaw(addButton(opts, line.newNum ? 'new' : 'old'))}<code class="code-inner">${htmlRaw(lineHtml[idx++])}</code></td></tr>`;
    }
  }
  return out + tailRow(hunks, false, opts);
}

function splitHalf(side: DiffSide, line: DiffLine | null, lineIdx: number, content: string, cellClass: string, opts: RowRenderOptions): string {
  const num = line ? (side === 'old' ? line.oldNum : line.newNum) || '' : '';
  const marker = line ? html`<span class="tw-font-mono" data-type-marker="${typeMarkers[line.type]}"></span>` : '';
  // like the legacy excerpts, expanded lines are only commented on the new side
  const button = line && !(side === 'old' && line.expanded) ? addButton(opts, side) : '';
  return html`<td class="lines-num lines-num-${side}${cellClass}" data-line-num="${num}"></td><td class="lines-escape lines-escape-${side}${cellClass}">${htmlRaw(line ? opts.escapeButtons[lineIdx] : '')}</td><td class="lines-type-marker lines-type-marker-${side}${cellClass}">${htmlRaw(marker)}</td><td class="chroma lines-code lines-code-${side}${cellClass}">${htmlRaw(button)}<code class="code-inner">${htmlRaw(content)}</code></td>`;
}

// a deleted line and its paired added line share a row
export function renderSplitRows(hunks: DiffHunk[], lineHtml: string[], pairs: Int32Array, opts: RowRenderOptions): string {
  let out = '';
  let idx = 0;
  for (const [hunkIdx, hunk] of hunks.entries()) {
    out += hunkRow(hunkIdx, hunk.header, true, opts);
    const base = idx;
    for (const line of hunk.lines) {
      const lineIdx = idx++;
      const pairIdx = pairs[lineIdx];
      if (line.type === 'add' && pairIdx !== -1) continue; // rendered with its deleted line
      let left: DiffLine | null = null;
      let right: DiffLine | null = null;
      let rightIdx = -1;
      if (line.type !== 'add') left = line;
      if (line.type !== 'del') {
        right = line;
        rightIdx = lineIdx;
      } else if (pairIdx !== -1) {
        right = hunk.lines[pairIdx - base];
        rightIdx = pairIdx;
      }
      const paired = line.type === 'del' && right !== null;
      out += html`<tr class="${line.type}-code${line.expanded ? ' line-expanded' : ''}" data-line-type="${line.type}" data-old="${left?.oldNum || ''}" data-new="${right?.newNum || ''}">${
        htmlRaw(splitHalf('old', left, lineIdx, left ? lineHtml[lineIdx] : '', paired ? ' del-code' : '', opts))
      }${htmlRaw(splitHalf('new', right, rightIdx, right ? lineHtml[rightIdx] : '', paired ? ' add-code' : '', opts))
      }</tr>`;
    }
  }
  return out + tailRow(hunks, true, opts);
}

// "same" lines are taken from the new side, so every line is highlighted with the context of its own side
export function highlightHunkLines(lines: DiffLine[], highlighter: LineHighlighter | null | undefined): Segment[][] {
  const newSide = highlightLines(highlighter, lines.filter((line) => line.type !== 'del').map((line) => line.content));
  const oldSide = highlightLines(highlighter, lines.filter((line) => line.type === 'del').map((line) => line.content));
  let newIdx = 0;
  let oldIdx = 0;
  return lines.map((line) => (line.type === 'del' ? oldSide[oldIdx++] : newSide[newIdx++]));
}

// the html of every line, a deleted line and its paired added line get their changed words marked
export function linesToHtml(lines: DiffLine[], segments: Segment[][], pairs: Int32Array, escapes: Array<LineEscapes | null>): string[] {
  const lineHtml: string[] = new Array(lines.length);
  for (const [idx, line] of lines.entries()) {
    if (line.type !== 'del' || pairs[idx] === -1) {
      lineHtml[idx] ??= segmentsToHtml(segments[idx], [], '', escapes[idx]?.escapes);
      continue;
    }
    const addIdx = pairs[idx];
    const ranges = wordDiffRanges(line.content, lines[addIdx].content);
    lineHtml[idx] = segmentsToHtml(segments[idx], ranges?.old, 'removed-code', escapes[idx]?.escapes);
    lineHtml[addIdx] = segmentsToHtml(segments[addIdx], ranges?.new, 'added-code', escapes[addIdx]?.escapes);
  }
  return lineHtml;
}

// mirrors RenderUnicodeEscapeToggleButton in modules/templates/util_render.go
export function escapeButton(escape: LineEscapes | null, titles: {ambiguousRunesLine: string; invisibleRunesLine: string}): string {
  if (!escape || (!escape.hasAmbiguous && !escape.hasInvisible)) return '';
  return html`<button type="button" class="toggle-escape-button btn interact-bg" title="${escape.hasAmbiguous ? titles.ambiguousRunesLine : titles.invisibleRunesLine}"></button>`;
}
