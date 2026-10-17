import type {DiffHunk, DiffSide} from './parse.ts';
import type {DiffStreamFile} from './stream.ts';
import type {LineHighlighter, Segment} from './highlight.ts';

// matches diffViewerConfig in routers/web/repo/diff_viewer.go
export type DiffViewerConfig = {
  streamUrl: string;
  commentsUrl?: string;
  issueLink?: string;
  repoLink?: string; // to browse the commits, the wiki can't
  beforeCommitId?: string;
  afterCommitId: string;
  editFileLink?: string;
  editReturnUri?: string;
  isSplit: boolean;
  canReviewFiles: boolean;
  maxFileSize: number;
  csvMaxFileSize: number;
  disableHighlight: boolean;
  detectAmbiguous: boolean;
};

export type DiffViewerLocale = Record<string, string>;

export type DiffFileEntry = {
  file: DiffStreamFile;
  hunks: DiffHunk[];
  missing: boolean; // listed by the file tree but not streamed, because the diff was truncated or it only has whitespace changes
  folded: boolean;
  element: HTMLElement | null; // built once and kept while scrolled out, so remounting doesn't flash
  highlighter: LineHighlighter | null | undefined; // undefined while loading
  renderedLines: {html: string[]; escapeButtons: string[]} | null; // in hunk order, escapeButtons are the unicode escape toggles
  linePairs: Int32Array | null;
  content: {old: string[] | null; new: string[]} | null; // the full file, loaded when expanding the context
  lfsState: 'loading' | 'tooLarge' | 'binary' | 'diff' | 'failed' | null; // the diff of the LFS objects instead of their pointers
  contentSegments: {old: Segment[][] | null; new: Segment[][]} | null;
  rowIndex: Record<DiffSide, Map<number, number>> | null; // the row of each line number, for ordering conversations
};
