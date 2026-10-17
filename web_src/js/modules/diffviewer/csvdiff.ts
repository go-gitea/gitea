import type {DiffHunk} from './parse.ts';
import {html, htmlEscape} from '../../utils/html.ts';
import {extname} from '../../utils.ts';

// a parse error at a 1-based position, fieldCount marks a row with a wrong number of fields
export class CsvParseError extends Error {
  line: number;
  column: number;
  fieldCount: boolean;
  constructor(line: number, column: number, fieldCount = false) {
    super(`CSV parse error at line ${line}, column ${column}`);
    this.line = line;
    this.column = column;
    this.fieldCount = fieldCount;
  }
}

// parses CSV like Go's encoding/csv
export function parseCsv(text: string, delimiter: string, {trimLeadingSpace = delimiter !== '\t' && delimiter !== ' ', sameFieldCount = false} = {}): string[][] {
  const rows: string[][] = [];
  const input = text.replaceAll('\r\n', '\n');
  let pos = 0;
  let line = 1;
  let lineStart = 0;
  const error = (at: number, fieldCount = false) => new CsvParseError(line, at - lineStart + 1, fieldCount);
  while (pos < input.length) {
    if (input[pos] === '\n') { // empty lines are skipped
      pos++;
      line++;
      lineStart = pos;
      continue;
    }
    const row: string[] = [];
    const rowLine = line;
    for (;;) {
      if (trimLeadingSpace) {
        while (input[pos] === ' ' || input[pos] === '\t') pos++;
      }
      if (input[pos] === '"') {
        let field = '';
        pos++;
        for (;;) {
          const quote = input.indexOf('"', pos);
          if (quote === -1) throw error(pos);
          for (let i = pos; i < quote; i++) {
            if (input[i] === '\n') {
              line++;
              lineStart = i + 1;
            }
          }
          field += input.slice(pos, quote);
          pos = quote + 1;
          if (input[pos] === '"') {
            field += '"';
            pos++;
          } else if (pos >= input.length || input[pos] === delimiter || input[pos] === '\n') {
            break;
          } else {
            throw error(pos);
          }
        }
        row.push(field);
      } else {
        let end = pos;
        while (end < input.length && input[end] !== delimiter && input[end] !== '\n') {
          if (input[end] === '"') throw error(end);
          end++;
        }
        row.push(input.slice(pos, end));
        pos = end;
      }
      if (input[pos] === delimiter) {
        pos++;
        continue;
      }
      if (input[pos] === '\n') {
        pos++;
        line++;
        lineStart = pos;
      }
      break;
    }
    if (sameFieldCount && rows.length && row.length !== rows[0].length) throw new CsvParseError(rowLine, 1, true);
    rows.push(row);
  }
  return rows;
}

const maxGuessLines = 10;
const guessSampleSize = 10 * 1000;
const beforeAfterQuotesRe = /([,@\t;|]?) *(?:"[^"]*")+([,@\t;|]?)/;

// like determineDelimiter in modules/csv/csv.go
export function determineDelimiter(filename: string, text: string): string {
  if (extname(filename).toLowerCase() === '.tsv') return '\t';
  const sample = text.slice(0, guessSampleSize);
  const quoted = beforeAfterQuotesRe.exec(sample);
  if (quoted?.[1]) return quoted[1];
  if (quoted?.[2]) return quoted[2];
  let lines = sample.replace(/"[^"]*"/g, '').split('\n');
  if (lines.length > maxGuessLines) {
    lines = lines.slice(0, maxGuessLines);
  } else if (lines.length > 1 && text.length >= guessSampleSize) {
    lines = lines.slice(0, -1); // the last line is probably truncated
  }
  let best = ',';
  let bestCount = 0;
  for (const delimiter of [',', '\t', ';', '|', '@']) {
    try {
      const rows = parseCsv(lines.join('\n'), delimiter, {trimLeadingSpace: false, sameFieldCount: true});
      if (rows.length && rows[0].length > bestCount) {
        best = delimiter;
        bestCount = rows[0].length;
      }
    } catch {}
  }
  return best;
}

// the types of a cell, 0 is a cell without a value on one side
const cellUnchanged = 1;
const cellChanged = 2;
const cellAdd = 3;
const cellDel = 4;
const cellMovedUnchanged = 5;
const cellMovedChanged = 6;

type TableDiffCell = {left: string; right: string; type: number};
type TableDiffRow = {rowIdx: number; cells: Array<TableDiffCell | null>};
type TableDiffSection = TableDiffRow[];

const unmapped = -1;
const rowsToInspect = 9; // the rows after the header compared to map columns by content, whether they exist or not
const minRatioToMatch = 0.8;

function columnMapping(baseRows: string[][], headRows: string[][]): [number[], number[]] {
  const baseRow = baseRows[0] ?? [];
  const headRow = headRows[0] ?? [];
  const base2Head = baseRow.map(() => unmapped);
  const head2Base = headRow.map(() => unmapped);
  for (const [i, baseCell] of baseRow.entries()) {
    const j = headRow.findIndex((headCell, idx) => head2Base[idx] === unmapped && headCell === baseCell);
    if (j !== -1) {
      base2Head[i] = j;
      head2Base[j] = i;
    }
  }
  const mapByContent = (aRows: string[][], a2b: number[], bRows: string[][], b2a: number[]) => {
    for (let i = 0; i < a2b.length; i++) {
      for (let j = 0; a2b[i] === unmapped && j < b2a.length; j++) {
        if (b2a[j] !== unmapped) continue;
        let same = 0;
        for (let row = 1; row <= rowsToInspect; row++) {
          const aCell = aRows[row]?.[i];
          if (aCell !== undefined && aCell === bRows[row]?.[j]) same++;
        }
        if (same / rowsToInspect > minRatioToMatch) {
          a2b[i] = j;
          b2a[j] = i;
        }
      }
    }
  };
  mapByContent(baseRows, base2Head, headRows, head2Base);
  mapByContent(headRows, head2Base, baseRows, base2Head);
  return [base2Head, head2Base];
}

// pairs the old and new line numbers of the changed lines
function mergeLines(hunk: DiffHunk): Array<[number, number]> {
  const result: Array<[number, number]> = [];
  for (const line of hunk.lines) {
    if (line.oldNum === 0 && result.length && result.at(-1)![1] === 0) {
      let idx = result.length;
      while (idx > 0 && result[idx - 1][1] === 0) idx--;
      result[idx][1] = line.newNum;
      continue;
    }
    result.push([line.oldNum, line.newNum]);
  }
  return result;
}

function singleSideDiff(rows: string[][], type: number): TableDiffSection[] {
  return [rows.map((row, idx) => ({rowIdx: idx + 1, cells: row.map((cell) => ({
    left: type === cellDel ? cell : '', right: type === cellAdd ? cell : '', type,
  }))}))];
}

// the cell diff of both versions, rows are matched by the line numbers of the hunks
export function createCsvDiff(baseRows: string[][] | null, headRows: string[][] | null, hunks: DiffHunk[]): TableDiffSection[] {
  if (!baseRows) return singleSideDiff(headRows ?? [], cellAdd);
  if (!headRows) return singleSideDiff(baseRows, cellDel);
  const [a2b, b2a] = columnMapping(baseRows, headRows);
  const unmappedCount = (mapping: number[]) => mapping.filter((col) => col === unmapped).length;
  const colCount = a2b.length < b2a.length ? b2a.length + unmappedCount(a2b) : a2b.length + unmappedCount(b2a);

  const createRow = (aLine: number, bLine: number): TableDiffRow | null => {
    const aRow = aLine > 0 ? baseRows[aLine - 1] ?? null : null;
    const bRow = bLine > 0 ? headRows[bLine - 1] ?? null : null;
    if (!aRow && !bRow) return null;
    const cells: Array<TableDiffCell | null> = new Array(colCount).fill(null);
    let aIndex = 0;
    let bIndex = 0;
    let colsAdded = 0;
    let colsDeleted = 0;
    while (aIndex < a2b.length || bIndex < b2a.length) {
      while (aIndex < a2b.length && a2b[aIndex] === unmapped && (bIndex >= b2a.length || aIndex <= bIndex)) {
        cells[bIndex + colsDeleted] = {left: aRow?.[aIndex] ?? '', right: '', type: cellDel};
        aIndex++;
        colsDeleted++;
      }
      while (aIndex < a2b.length && a2b[aIndex] !== unmapped) aIndex++;
      while (bIndex < b2a.length && b2a[bIndex] === unmapped && (aIndex >= a2b.length || bIndex < aIndex)) {
        cells[bIndex + colsDeleted] = {left: '', right: bRow?.[bIndex] ?? '', type: bRow ? cellAdd : cellDel};
        bIndex++;
        colsAdded++;
      }
      while (bIndex < b2a.length && b2a[bIndex] !== unmapped && (aIndex >= a2b.length || bIndex < aIndex)) {
        const aCell = aRow?.[b2a[bIndex]];
        const bCell = bRow?.[bIndex];
        const cell: TableDiffCell = {left: aCell ?? '', right: bCell ?? '', type: 0};
        if (!aRow) cell.type = cellAdd;
        if (!bRow) cell.type = cellDel;
        if (aCell !== undefined && bCell !== undefined) {
          const moved = bIndex + colsDeleted !== b2a[bIndex] + colsAdded;
          if (aCell !== bCell) {
            cell.type = moved ? cellMovedChanged : cellChanged;
          } else {
            cell.type = moved ? cellMovedUnchanged : cellUnchanged;
            cell.left = '';
          }
        }
        cells[bIndex + colsDeleted] = cell;
        bIndex++;
      }
    }
    return {rowIdx: bLine, cells};
  };

  const sections: TableDiffSection[] = [];
  for (const [hunkIdx, hunk] of hunks.entries()) {
    const rows: TableDiffRow[] = [];
    for (const [lineIdx, [aLine, bLine]] of mergeLines(hunk).entries()) {
      if (hunkIdx === 0 && lineIdx === 0 && (aLine !== 1 || bLine !== 1)) {
        const header = createRow(1, 1);
        if (header) rows.push(header);
      }
      const row = createRow(aLine, bLine);
      if (row) rows.push(row);
    }
    if (rows.length) sections.push(rows);
  }
  return sections;
}

function cellHtml(cell: TableDiffCell | null, tag: 'th' | 'td'): string {
  if (!cell) return `<${tag}></${tag}>`;
  const removed = html`<span class="removed-code">${cell.left}</span>`;
  const added = html`<span class="added-code">${cell.right}</span>`;
  switch (cell.type) {
    case cellChanged: return `<${tag} class="modified">${removed} ${added}</${tag}>`;
    case cellAdd: return `<${tag} class="added">${added}</${tag}>`;
    case cellDel: return `<${tag} class="removed">${removed}</${tag}>`;
    case cellMovedUnchanged: return `<${tag} class="moved">${htmlEscape(cell.right)}</${tag}>`;
    case cellMovedChanged: return `<${tag} class="moved">${removed} ${added}</${tag}>`;
    default: return `<${tag}>${htmlEscape(cell.right)}</${tag}>`;
  }
}

// the first row of the first section is the header
export function renderCsvDiff(sections: TableDiffSection[]): string {
  let out = '<table class="data-table">';
  for (const [sectionIdx, rows] of sections.entries()) {
    out += sectionIdx > 0 ? '<tbody class="section">' : '<tbody>';
    for (const [rowIdx, row] of rows.entries()) {
      const tag = sectionIdx === 0 && rowIdx === 0 ? 'th' : 'td';
      out += `<tr><${tag} class="line-num">${row.rowIdx || ''}</${tag}>${row.cells.map((cell) => cellHtml(cell, tag)).join('')}</tr>`;
    }
    out += '</tbody>';
  }
  return `${out}</table>`;
}
