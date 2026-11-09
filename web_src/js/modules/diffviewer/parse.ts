export type DiffSide = 'old' | 'new';

export type DiffLine = {
  type: 'same' | 'add' | 'del';
  oldNum: number; // 0 for added lines
  newNum: number; // 0 for deleted lines
  content: string;
  expanded?: boolean; // a context line shown by expanding
};

export type DiffHunk = {
  header: string;
  oldStart: number;
  oldCount: number;
  newStart: number;
  newCount: number;
  lines: DiffLine[];
};

const hunkHeaderRe = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

export function hunkHeader({oldStart, oldCount, newStart, newCount}: DiffHunk, suffix = ''): string {
  return `@@ -${oldStart},${oldCount} +${newStart},${newCount} @@${suffix}`;
}

// the lines of a text, without the CR of CRLF line endings
export function splitLines(text: string): string[] {
  const lines = text.split('\n').map((line) => (line.endsWith('\r') ? line.slice(0, -1) : line));
  if (text.endsWith('\n') || !text) lines.pop();
  return lines;
}

// parses the hunks of a single file's unified diff, everything before the first hunk header is ignored
export function parseHunks(text: string): DiffHunk[] {
  const hunks: DiffHunk[] = [];
  let hunk: DiffHunk | null = null;
  let oldNum = 0;
  let newNum = 0;
  for (const line of splitLines(text)) {
    const match = hunkHeaderRe.exec(line);
    if (match) {
      oldNum = Number(match[1]);
      newNum = Number(match[3]);
      hunk = {header: line, oldStart: oldNum, oldCount: Number(match[2] ?? 1), newStart: newNum, newCount: Number(match[4] ?? 1), lines: []};
      hunks.push(hunk);
      continue;
    }
    if (!hunk) continue;
    const marker = line[0];
    if (marker === '+') {
      hunk.lines.push({type: 'add', oldNum: 0, newNum: newNum++, content: line.slice(1)});
    } else if (marker === '-') {
      hunk.lines.push({type: 'del', oldNum: oldNum++, newNum: 0, content: line.slice(1)});
    } else if (marker === ' ') {
      hunk.lines.push({type: 'same', oldNum: oldNum++, newNum: newNum++, content: line.slice(1)});
    }
    // "\ No newline at end of file" is skipped
  }
  return hunks;
}
