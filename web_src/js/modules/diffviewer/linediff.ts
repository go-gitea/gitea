import {diffArrays} from 'diff';
import {hunkHeader, type DiffHunk, type DiffLine} from './parse.ts';

const contextSize = 3; // like "git diff"
const maxEditLength = 1000;

function diffLines(oldLines: string[], newLines: string[]): DiffLine[] {
  // very different content would take too long, it is replaced as a whole
  const changes = diffArrays(oldLines, newLines, {maxEditLength}) ?? [{value: oldLines, removed: true, added: false}, {value: newLines, removed: false, added: true}];
  const lines: DiffLine[] = [];
  let oldNum = 0;
  let newNum = 0;
  for (const change of changes) {
    for (const content of change.value) {
      if (change.removed) lines.push({type: 'del', oldNum: ++oldNum, newNum: 0, content});
      else if (change.added) lines.push({type: 'add', oldNum: 0, newNum: ++newNum, content});
      else lines.push({type: 'same', oldNum: ++oldNum, newNum: ++newNum, content});
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
