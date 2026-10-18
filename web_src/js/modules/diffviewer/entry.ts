import type {DiffHunk, DiffLine, DiffSide} from './parse.ts';
import type {DiffStreamFile} from './stream.ts';
import type {LineHighlighter, Segment} from './highlight.ts';
import {gapsOf, type Gap} from './expand.ts';
import {hunksLineCount, pairChangedLines} from './render.ts';

export type FileContent = {old: string[] | null; new: string[]}; // the lines of the whole file, old is null for added files
export type LfsState = 'loading' | 'tooLarge' | 'binary' | 'diff' | 'failed' | null;
export type RenderedLines = {html: string[]; escapeButtons: string[]}; // in hunk order, escapeButtons are the unicode escape toggles

// a file of the diff and the data derived from its hunks, which is dropped when the hunks change
export class DiffFile {
  file: DiffStreamFile;
  missing: boolean; // listed by the file tree but not streamed, because the diff was truncated or it only has whitespace changes
  folded: boolean;
  element: HTMLElement | null = null; // built once and kept while scrolled out, so remounting doesn't flash
  highlighter: LineHighlighter | null | undefined = undefined; // undefined while loading
  lfsState: LfsState = null; // the diff of the LFS objects instead of their pointers
  content: FileContent | null = null; // for highlighting and expanding the context
  contentPromise: Promise<FileContent> | null = null; // while it loads
  renderedLines: RenderedLines | null = null;
  contentSegments: {old: Segment[][] | null; new: Segment[][]} | null = null; // the highlighted content
  #hunks: DiffHunk[];
  #linePairs: Int32Array | null = null;
  #rowIndex: Record<DiffSide, Map<number, number>> | null = null;
  #rowCount: number | null = null;
  #gaps: Gap[] | null = null;

  constructor(file: DiffStreamFile, hunks: DiffHunk[], missing: boolean, folded: boolean) {
    this.file = file;
    this.#hunks = hunks;
    this.missing = missing;
    this.folded = folded;
  }

  get hunks(): DiffHunk[] {
    return this.#hunks;
  }

  set hunks(hunks: DiffHunk[]) {
    this.#hunks = hunks;
    this.invalidateRows();
  }

  get lines(): DiffLine[] {
    return this.#hunks.flatMap((hunk) => hunk.lines);
  }

  // after the hunks were changed in place, like by expanding a gap
  invalidateRows(): void {
    this.renderedLines = null;
    this.#linePairs = null;
    this.#rowIndex = null;
    this.#rowCount = null;
    this.#gaps = null;
  }

  // after the highlighter arrived, the lines are rendered again with it
  invalidateHighlighting(): void {
    this.renderedLines = null;
    this.contentSegments = null;
  }

  setContent(content: FileContent): void {
    this.content = content;
    this.contentSegments = null;
    this.#gaps = null; // the tail gap ends at the known line count now
  }

  changeCounts(): {additions: number; deletions: number} {
    let additions = 0;
    let deletions = 0;
    for (const line of this.lines) {
      if (line.type === 'add') additions++;
      else if (line.type === 'del') deletions++;
    }
    return {additions, deletions};
  }

  // the paired line of each line in hunk order, see pairChangedLines
  linePairs(): Int32Array {
    this.#linePairs ??= pairChangedLines(this.#hunks);
    return this.#linePairs;
  }

  // the row of a line number in the rendered table, or -1, for ordering conversations
  rowIndex(side: DiffSide, line: number): number {
    if (!this.#rowIndex) {
      this.#rowIndex = {old: new Map(), new: new Map()};
      let row = 0;
      for (const hunk of this.#hunks) {
        row++; // the hunk header
        for (const diffLine of hunk.lines) {
          if (diffLine.oldNum) this.#rowIndex.old.set(diffLine.oldNum, row);
          if (diffLine.newNum) this.#rowIndex.new.set(diffLine.newNum, row);
          row++;
        }
      }
    }
    return this.#rowIndex[side].get(line) ?? -1;
  }

  // the rows of the rendered table for the height estimate, the layout doesn't change while the viewer is shown
  rowCount(isSplit: boolean): number {
    if (this.#rowCount === null) {
      let rows = hunksLineCount(this.#hunks);
      if (isSplit) {
        for (const pair of this.linePairs()) if (pair !== -1) rows -= 0.5; // a deleted line and its paired added line share a row
      }
      this.#rowCount = rows;
    }
    return this.#rowCount;
  }

  // the hidden lines between the hunks, none when the file can't be expanded
  gaps(expandable: boolean): Gap[] {
    this.#gaps ??= expandable ? gapsOf(this.#hunks, this.content?.new.length ?? null) : [];
    return this.#gaps;
  }
}
