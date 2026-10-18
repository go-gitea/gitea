import type {DiffSide} from './parse.ts';
import type {SvgName} from '../../svg.ts';

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

// matches diffViewerComment in routers/web/repo/pull_files_comments.go
export type ViewerComment = {
  id: number;
  path: string;
  side: DiffSide;
  line: number;
  poster: {name: string; displayName: string; homeLink?: string; avatar: string};
  originalAuthor?: string;
  body: string;
  bodyHtml: string;
  contentVersion: number;
  created: number;
  outdated: boolean;
  pending: boolean;
  reviewId: number;
  resolved: boolean;
  resolvedBy: string;
  canEdit: boolean;
  canBlockUser: boolean;
  canBlockOrg: boolean;
  reactions: Array<{content: string; emojiHtml: string; count: number; users: string; moreCount: number; hasReacted: boolean}>;
  attachments: Array<{name: string; size: number; downloadUrl: string; isImage: boolean; inContent: boolean}>;
};

// the response of the comments route in routers/web/repo/pull_files_comments.go
export type CommentsData = {
  isSigned: boolean;
  canComment: boolean;
  canResolve: boolean;
  canReferenceIssue: boolean;
  hasPendingReview: boolean;
  pendingCount: number;
  issueHtmlUrl: string;
  repoLink: string;
  orgBlockLink?: string;
  originalUrl?: string;
  originalHostname?: string;
  migrationIcon?: SvgName;
  reactions: Array<{content: string; emojiHtml: string}>;
  comments: ViewerComment[];
};
