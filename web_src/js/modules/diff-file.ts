import {reactive} from 'vue';
import type {Reactive} from 'vue';
import {basename, extname} from '../utils.ts';

const {pageData} = window.config;

// matches statusFromLetter in services/gitdiff/git_diff_tree.go
export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechanged' | 'unmerged' | 'unknown';

export type DiffTreeEntry = {
  FullName: string,
  OldFullName: string,
  DisplayName: string,
  NameHash: string,
  DiffStatus: DiffStatus,
  EntryMode: string,
  IsViewed: boolean,
  HasChanged: boolean, // since the last review
  Children: DiffTreeEntry[] | null,
  FileIcon: string,
  ParentEntry?: DiffTreeEntry,
};

export type DiffFileTreeData = {
  TreeRoot: DiffTreeEntry,
};

// activeExtensions: 'all' = no filter (every extension passes); string[] = exact set of extensions allowed (empty = nothing passes).
type ExtensionFilter = 'all' | string[];

export type DiffStatusGroup = 'added' | 'modified' | 'deleted' | 'renamed';
const diffStatusGroups: DiffStatusGroup[] = ['added', 'modified', 'deleted', 'renamed'];

// the code comment threads of a file, shown in the comments tab of a pull request
export type DiffTreeCommentThread = {
  id: number; // of the first comment, its anchor is the target of the thread's link
  line: number;
  posterName: string;
  avatarUrl: string;
  excerpt: string;
  count: number;
  resolved: boolean;
  outdated: boolean;
  pending: boolean;
};
export type DiffTreeCommentFile = {name: string; threads: DiffTreeCommentThread[]};

type DiffFileTree = {
  folderIcon: string;
  folderOpenIcon: string;
  diffFileTree: DiffFileTreeData;
  fullNameMap: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
  filenameFilterQuery: string;
  activeExtensions: ExtensionFilter;
  activeStatuses: 'all' | DiffStatusGroup[];
  commentedOnly: boolean;
  commentFiles: DiffTreeCommentFile[] | null; // null on pages without code comments
  activeTab: 'files' | 'comments';
};

type DiffExtensionStats = {
  ext: string,
  count: number,
};

export type DiffFileFilterLocale = {
  filterOptions: string,
  fileStatus: string,
  fileStatusAdded: string,
  fileStatusModified: string,
  fileStatusDeleted: string,
  fileStatusRenamed: string,
  hasComments: string,
  fileExtensions: string,
  noFileExtension: string,
  dotfileExtension: string,
  allFileExtensions: string,
};

export type DiffFileTreeLocale = DiffFileFilterLocale & {
  filterFiles: string,
  filterFilesClear: string,
  files: string,
  comments: string,
  noComments: string,
  outdated: string,
  pending: string,
  resolved: string,
};

let diffTreeStoreReactive: Reactive<DiffFileTree>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactiveDiffTreeStore(pageData.DiffFileTree!, pageData.FolderIcon!, pageData.FolderOpenIcon!);
    const knownExtensions = getDiffTreeExtensionStats(diffTreeStoreReactive).map((stat) => stat.ext);
    diffTreeStoreReactive.activeExtensions = extensionFilterFromUrl(window.location.search, knownExtensions);
  }
  return diffTreeStoreReactive;
}

export function diffTreeStoreSetViewed(store: Reactive<DiffFileTree>, fullName: string, viewed: boolean) {
  const entry = store.fullNameMap[fullName];
  if (!entry) return;
  entry.IsViewed = viewed;
  entry.HasChanged = false;
  for (let parent = entry.ParentEntry; parent; parent = parent.ParentEntry) {
    parent.IsViewed = isEntryViewed(parent);
  }
}

function fillFullNameMap(map: Record<string, DiffTreeEntry>, entry: DiffTreeEntry) {
  map[entry.FullName] = entry;
  if (!entry.Children) return;
  entry.IsViewed = isEntryViewed(entry);
  for (const child of entry.Children) {
    child.ParentEntry = entry;
    fillFullNameMap(map, child);
  }
}

export function reactiveDiffTreeStore(data: DiffFileTreeData, folderIcon: string, folderOpenIcon: string): Reactive<DiffFileTree> {
  const store = reactive<DiffFileTree>({
    diffFileTree: data,
    folderIcon,
    folderOpenIcon,
    fileTreeIsVisible: false,
    selectedItem: '',
    filenameFilterQuery: '',
    activeExtensions: 'all',
    activeStatuses: 'all',
    commentedOnly: false,
    commentFiles: null,
    activeTab: 'files',
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

export const extDotfile = 'dotfile'; // bucket for ".gitignore" and friends, real extensions always start with a dot

const urlParamFileFilters = 'file-filters[]'; // same parameter GitHub uses, lists the selected extensions
const urlValueNoExtension = 'noextension';

export function extensionFilterFromUrl(search: string, knownExtensions: string[]): ExtensionFilter {
  const params = new URLSearchParams(search);
  if (!params.has(urlParamFileFilters)) return 'all';
  const extensions = params.getAll(urlParamFileFilters)
    .filter(Boolean)
    .map((ext) => ext === urlValueNoExtension ? '' : ext)
    .filter((ext) => knownExtensions.includes(ext));
  return extensions.length === knownExtensions.length ? 'all' : extensions;
}

export function extensionFilterToUrl(filter: ExtensionFilter, url: string): string {
  const parsed = new URL(url);
  parsed.searchParams.delete(urlParamFileFilters);
  if (filter !== 'all') {
    if (!filter.length) parsed.searchParams.append(urlParamFileFilters, '');
    for (const ext of filter) parsed.searchParams.append(urlParamFileFilters, ext || urlValueNoExtension);
  }
  return parsed.href;
}

function getFileExtension(filename: string): string {
  const ext = extname(filename).toLowerCase();
  if (ext) return ext;
  return basename(filename).startsWith('.') ? extDotfile : '';
}

function extensionRank(ext: string): number {
  if (!ext) return 2;
  return ext === extDotfile ? 1 : 0;
}

export function diffTreeFiles(store: Reactive<DiffFileTree>): DiffTreeEntry[] {
  return Object.values(store.fullNameMap).filter((entry) => entry.Children === null && entry.FullName);
}

export function getDiffTreeExtensionStats(store: Reactive<DiffFileTree>): DiffExtensionStats[] {
  const extensionMap = new Map<string, number>();
  for (const entry of diffTreeFiles(store)) {
    const ext = getFileExtension(entry.FullName);
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }
  return Array.from(extensionMap, ([ext, count]) => ({ext, count}))
    .sort((a, b) => extensionRank(a.ext) - extensionRank(b.ext) || a.ext.localeCompare(b.ext));
}

// copies have an old name like renames, type changes are modifications
function diffStatusGroup(status: DiffStatus): DiffStatusGroup {
  if (status === 'added' || status === 'deleted' || status === 'renamed') return status;
  return status === 'copied' ? 'renamed' : 'modified';
}

export function getDiffTreeStatusStats(store: Reactive<DiffFileTree>): Array<{status: DiffStatusGroup, count: number}> {
  const counts = new Map<DiffStatusGroup, number>();
  for (const entry of diffTreeFiles(store)) {
    const status = diffStatusGroup(entry.DiffStatus);
    counts.set(status, (counts.get(status) ?? 0) + 1);
  }
  return diffStatusGroups.filter((status) => counts.has(status)).map((status) => ({status, count: counts.get(status)!}));
}

function buildFilter(store: Reactive<DiffFileTree>) {
  const query = store.filenameFilterQuery.trim().toLowerCase();
  const exts = store.activeExtensions === 'all' ? null : new Set(store.activeExtensions);
  const statuses = store.activeStatuses === 'all' ? null : new Set(store.activeStatuses);
  const commented = store.commentedOnly && store.commentFiles ? new Set(store.commentFiles.map((file) => file.name)) : null;
  if (!query && !exts && !statuses && !commented) return null;
  return (entry: DiffTreeEntry) => {
    if (query && !entry.FullName.toLowerCase().includes(query) && !entry.OldFullName.toLowerCase().includes(query)) return false;
    if (statuses && !statuses.has(diffStatusGroup(entry.DiffStatus))) return false;
    if (commented && !commented.has(entry.FullName)) return false;
    return !exts || exts.has(getFileExtension(entry.FullName));
  };
}

// Children===null marks a file leaf; everything else (incl. the root, which has EntryMode="") is recursed into.
export function filterDiffTree(store: Reactive<DiffFileTree>): DiffTreeEntry | null {
  const matches = buildFilter(store);
  if (!matches) return store.diffFileTree.TreeRoot;
  const visit = (entry: DiffTreeEntry): DiffTreeEntry | null => {
    if (entry.Children === null) return matches(entry) ? entry : null;
    const children = entry.Children.map(visit).filter((child): child is DiffTreeEntry => child !== null);
    if (!children.length) return null;
    return {...entry, Children: children};
  };
  return visit(store.diffFileTree.TreeRoot);
}

// the names of the files the filters show, null if nothing is filtered
export function filteredFileNames(store: Reactive<DiffFileTree>): Set<string> | null {
  const matches = buildFilter(store);
  if (!matches) return null;
  return new Set(diffTreeFiles(store).filter(matches).map((entry) => entry.FullName));
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    return entry.Children.every((child) => child.IsViewed);
  }
  return entry.IsViewed;
}
