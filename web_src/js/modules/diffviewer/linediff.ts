import {diffSequences} from './myers.ts';
import {hunkHeader, type DiffHunk, type DiffLine} from './parse.ts';

const contextSize = 3; // like "git diff"

function diffLines(oldLines: string[], newLines: string[]): DiffLine[] {
  const lines: DiffLine[] = [];
  for (const op of diffSequences(oldLines, newLines)) {
    if (op.type === 'equal') {
      for (let i = 0; i < op.aEnd - op.aStart; i++) {
        lines.push({type: 'same', oldNum: op.aStart + i + 1, newNum: op.bStart + i + 1, content: newLines[op.bStart + i]});
      }
    } else if (op.type === 'delete') {
      for (let i = op.aStart; i < op.aEnd; i++) lines.push({type: 'del', oldNum: i + 1, newNum: 0, content: oldLines[i]});
    } else {
      for (let i = op.bStart; i < op.bEnd; i++) lines.push({type: 'add', oldNum: 0, newNum: i + 1, content: newLines[i]});
    }
  }
  return lines;
}

// builds the hunks of a unified diff between two versions of a file, like "git diff" with 3 lines of context
export function buildHunks(oldLines: string[], newLines: string[]): DiffHunk[] {
  const lines = diffLines(oldLines, newLines);
  // groups of changed lines which are close enough to share their context
  const groups: Array<[first: number, last: number]> = [];
  for (const [idx, line] of lines.entries()) {
    if (line.type === 'same') continue;
    const last = groups.at(-1);
    if (last && idx - last[1] - 1 <= 2 * contextSize) last[1] = idx;
    else groups.push([idx, idx]);
  }
  const hunks: DiffHunk[] = [];
  let pos = 0;
  let oldBefore = 0;
  let newBefore = 0;
  for (const [first, last] of groups) {
    const start = Math.max(0, first - contextSize);
    const end = Math.min(lines.length, last + contextSize + 1);
    for (; pos < start; pos++) {
      if (lines[pos].type !== 'add') oldBefore++;
      if (lines[pos].type !== 'del') newBefore++;
    }
    const hunkLines = lines.slice(start, end);
    const oldCount = hunkLines.filter((line) => line.type !== 'add').length;
    const newCount = hunkLines.filter((line) => line.type !== 'del').length;
    // a side without lines starts at the line before the hunk
    const oldStart = oldCount ? oldBefore + 1 : oldBefore;
    const newStart = newCount ? newBefore + 1 : newBefore;
    const hunk = {header: '', oldStart, oldCount, newStart, newCount, lines: hunkLines};
    hunk.header = hunkHeader(hunk);
    hunks.push(hunk);
  }
  return hunks;
}
