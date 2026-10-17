import type {DiffHunk, DiffSide} from './parse.ts';
import type {DiffStreamFile} from './stream.ts';
import type {LineHighlighter, Segment} from './highlight.ts';
import type {Gap} from './expand.ts';

// matches diffViewerConfig in routers/web/repo/diff_viewer.go
export type DiffViewerConfig = {
  streamUrl: string;
  commentsUrl?: string;
  issueLink?: string;
  repoLink?: string; // to browse the commits, the wiki can't
  beforeCommitId?: string;
  afterCommitId: string;
  headCommitId?: string; // of the pull request, new comments are attached to it
  editFileLink?: string;
  editReturnUri?: string;
  isSplit: boolean;
  canReviewFiles: boolean;
  maxFileSize: number;
  csvMaxFileSize: number;
  disableHighlight: boolean;
  detectAmbiguous: boolean;
};

// the keys of the "data-locale" dict in templates/repo/diff/box.tmpl
export type DiffViewerLocale = Record<
  'loadFailed' | 'tooManyFiles' | 'load' | 'fileSuppressed' | 'bin' | 'binNotShown' | 'renamedNoChanges' |
  'modeChanged' | 'whitespaceOnly' | 'submoduleAdded' | 'submoduleDeleted' | 'submoduleUpdated' |
  'copyPath' | 'viewed' | 'changedSinceLastReview' | 'addLineComment' | 'singleComment' | 'cancel' |
  'commentedAt' | 'outdated' | 'pending' | 'reply' | 'resolve' | 'unresolve' | 'resolvedBy' |
  'showResolved' | 'hideResolved' | 'previous' | 'next' | 'outdatedDescription' | 'pendingTooltip' |
  'review' | 'noContent' | 'migratedFrom' | 'copyLink' | 'copySource' | 'quoteReply' | 'referenceIssue' |
  'edit' | 'delete' | 'deleteConfirm' | 'blockUser' | 'blockOrg' | 'reactions' | 'reactionsMore' |
  'openAttachment' | 'startReview' | 'addReviewComment' | 'generated' | 'vendored' | 'protected' |
  'modeDirectory' | 'modeNormal' | 'modeExecutable' | 'modeSymlink' | 'modeSubmodule' | 'modeChangedFormat' |
  'escape' | 'unescape' | 'copyFilename' | 'viewFile' | 'editFile' | 'numComments1' | 'numComments' |
  'loading' | 'viewSource' | 'viewRendered' | 'sideBySide' | 'swipe' | 'overlay' | 'fileBefore' |
  'fileAfter' | 'imageWidth' | 'imageHeight' | 'byteSize' | 'csvTooLarge' | 'csvUnexpected' |
  'csvFieldCount' | 'ambiguousRunesLine' | 'invisibleRunesLine' | 'ambiguousCharacter',
  string>;

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
  gaps: Gap[] | null; // the hidden lines between the hunks
  rowCount: number | null; // of the rendered table, for the height estimate
};
