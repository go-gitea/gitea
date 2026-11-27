import {hunkHeader, type DiffHunk, type DiffLine, type DiffSide} from './parse.ts';

const expandChunkSize = 20;

export type ExpandDirection = 'up' | 'down' | 'all';

// the hidden lines above hunk "index", index === hunks.length is the tail after the last hunk, line ranges are inclusive
export type Gap = {index: number; newFrom: number; newTo: number; oldFrom: number; oldTo: number};

// a hunk without lines on a side starts after its "start" line on that side
const firstLine = (start: number, count: number) => (count === 0 ? start + 1 : start);
const lastLine = (start: number, count: number) => (count === 0 ? start : start + count - 1);

function hunkSuffix(hunk: DiffHunk): string {
  return /^@@ [^@]* @@(.*)$/.exec(hunk.header)![1];
}

function updateHeader(hunk: DiffHunk) {
  hunk.header = hunkHeader(hunk, hunkSuffix(hunk));
}

// git shows 3 context lines, so a last hunk with fewer after its changes ends at the end of the file
function reachesEnd(hunk: DiffHunk): boolean {
  let trailing = 0;
  for (let i = hunk.lines.length - 1; i >= 0 && hunk.lines[i].type === 'same'; i--) trailing++;
  return trailing < 3;
}

// the gaps between hunks, the tail ends at newLineCount, or is unbounded when the line count isn't known yet
export function gapsOf(hunks: DiffHunk[], newLineCount: number | null): Gap[] {
  const gaps: Gap[] = [];
  let prevNew = 0;
  let prevOld = 0;
  for (const [index, hunk] of hunks.entries()) {
    const newTo = firstLine(hunk.newStart, hunk.newCount) - 1;
    const oldTo = firstLine(hunk.oldStart, hunk.oldCount) - 1;
    if (newTo > prevNew) gaps.push({index, newFrom: prevNew + 1, newTo, oldFrom: prevOld + 1, oldTo});
    prevNew = lastLine(hunk.newStart, hunk.newCount);
    prevOld = lastLine(hunk.oldStart, hunk.oldCount);
  }
  const tailTo = newLineCount ?? (hunks.length && reachesEnd(hunks.at(-1)!) ? prevNew : Infinity);
  if (hunks.length && tailTo > prevNew) {
    gaps.push({index: hunks.length, newFrom: prevNew + 1, newTo: tailTo, oldFrom: prevOld + 1, oldTo: prevOld + tailTo - prevNew});
  }
  return gaps;
}

export function gapOfLine(gaps: Gap[], side: DiffSide, line: number): Gap | null {
  return gaps.find((gap) => (side === 'new' ? gap.newFrom <= line && line <= gap.newTo : gap.oldFrom <= line && line <= gap.oldTo)) ?? null;
}

function contextLines(gap: Gap, newFrom: number, newTo: number, newLines: string[]): DiffLine[] {
  const offset = gap.oldFrom - gap.newFrom;
  const lines: DiffLine[] = [];
  for (let newNum = newFrom; newNum <= newTo && newNum <= newLines.length; newNum++) {
    lines.push({type: 'same', oldNum: newNum + offset, newNum, content: newLines[newNum - 1], expanded: true});
  }
  return lines;
}

function appendLines(hunk: DiffHunk, lines: DiffLine[]) {
  if (!lines.length) return;
  if (hunk.newCount === 0) hunk.newStart++;
  if (hunk.oldCount === 0) hunk.oldStart++;
  hunk.lines.push(...lines);
  hunk.newCount += lines.length;
  hunk.oldCount += lines.length;
  updateHeader(hunk);
}

function prependLines(hunk: DiffHunk, lines: DiffLine[]) {
  if (!lines.length) return;
  hunk.newStart = firstLine(hunk.newStart, hunk.newCount) - lines.length;
  hunk.oldStart = firstLine(hunk.oldStart, hunk.oldCount) - lines.length;
  hunk.lines.unshift(...lines);
  hunk.newCount += lines.length;
  hunk.oldCount += lines.length;
  updateHeader(hunk);
}

// shows hidden lines of a gap as context lines, a gap between hunks merges them when it's fully shown
export function expandGap(hunks: DiffHunk[], gap: Gap, direction: ExpandDirection, newLines: string[]) {
  const newTo = Math.min(gap.newTo, newLines.length);
  const isTail = gap.index === hunks.length;
  if (direction === 'all' || newTo - gap.newFrom + 1 <= expandChunkSize) {
    const lines = contextLines(gap, gap.newFrom, newTo, newLines);
    if (gap.index === 0) {
      prependLines(hunks[0], lines);
    } else {
      const prev = hunks[gap.index - 1];
      appendLines(prev, lines);
      if (!isTail) {
        const next = hunks[gap.index];
        prev.lines.push(...next.lines);
        prev.newCount += next.newCount;
        prev.oldCount += next.oldCount;
        updateHeader(prev);
        hunks.splice(gap.index, 1);
      }
    }
  } else if (direction === 'down') {
    appendLines(hunks[gap.index - 1], contextLines(gap, gap.newFrom, gap.newFrom + expandChunkSize - 1, newLines));
  } else {
    prependLines(hunks[gap.index], contextLines(gap, newTo - expandChunkSize + 1, newTo, newLines));
  }
}

export function expandDirections(gap: Gap, hunkCount: number): ExpandDirection[] {
  if (gap.newTo - gap.newFrom + 1 <= expandChunkSize) return ['all'];
  if (gap.index === hunkCount) return ['down'];
  return gap.index === 0 ? ['up'] : ['down', 'up'];
}
