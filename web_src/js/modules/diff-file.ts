import {reactive} from 'vue';
import type {Reactive} from 'vue';
import {extname} from '../utils.ts';
import {toggleElem} from '../utils/dom.ts';

const {pageData} = window.config;

export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechange';

export type DiffTreeEntry = {
  FullName: string,
  DisplayName: string,
  NameHash: string,
  DiffStatus: DiffStatus,
  EntryMode: string,
  IsViewed: boolean,
  Children: DiffTreeEntry[] | null,
  FileIcon: string,
  ParentEntry?: DiffTreeEntry,
};

export type DiffFileTreeData = {
  TreeRoot: DiffTreeEntry,
};

// activeExtensions: 'all' = no filter (every extension passes); string[] = exact set of extensions allowed (empty = nothing passes).
export type ExtensionFilter = 'all' | string[];

type DiffFileTree = {
  folderIcon: string;
  folderOpenIcon: string;
  diffFileTree: DiffFileTreeData;
  fullNameMap: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
  filenameFilterQuery: string;
  activeExtensions: ExtensionFilter;
};

export type DiffExtensionStats = {
  ext: string,
  count: number,
};

let diffTreeStoreReactive: Reactive<DiffFileTree>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactiveDiffTreeStore(pageData.DiffFileTree!, pageData.FolderIcon!, pageData.FolderOpenIcon!);
  }
  return diffTreeStoreReactive;
}

export function diffTreeStoreSetViewed(store: Reactive<DiffFileTree>, fullName: string, viewed: boolean) {
  const entry = store.fullNameMap[fullName];
  if (!entry) return;
  entry.IsViewed = viewed;
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
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

export function getDiffTreeExtensionStats(store: Reactive<DiffFileTree>): DiffExtensionStats[] {
  const extensionMap = new Map<string, number>();
  for (const entry of Object.values(store.fullNameMap)) {
    if (entry.EntryMode === 'tree' || !entry.FullName) continue;
    const ext = extname(entry.FullName).toLowerCase();
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }
  return Array.from(extensionMap, ([ext, count]) => ({ext, count}))
    .sort((a, b) => b.count - a.count);
}

type DiffFilter = (filename: string) => boolean;

// Returns null when no filters are active, so callers can skip work entirely.
function buildFilter(store: Reactive<DiffFileTree>): DiffFilter | null {
  const query = store.filenameFilterQuery.trim().toLowerCase();
  const exts = store.activeExtensions === 'all' ? null : new Set(store.activeExtensions);
  if (!query && !exts) return null;
  return (filename) => {
    if (!filename) return false;
    if (query && !filename.toLowerCase().includes(query)) return false;
    return !exts || exts.has(extname(filename).toLowerCase());
  };
}

// Children===null marks a file leaf; everything else (incl. the root, which has EntryMode="") is recursed into.
export function filterDiffTree(store: Reactive<DiffFileTree>): DiffTreeEntry | null {
  const matches = buildFilter(store);
  if (!matches) return store.diffFileTree.TreeRoot;
  const visit = (entry: DiffTreeEntry): DiffTreeEntry | null => {
    if (entry.Children === null) return matches(entry.FullName) ? entry : null;
    const children = entry.Children.map(visit).filter((child): child is DiffTreeEntry => child !== null);
    if (!children.length) return null;
    return {...entry, Children: children};
  };
  return visit(store.diffFileTree.TreeRoot);
}

export function applyFiltersToFileBoxes(store: Reactive<DiffFileTree>) {
  const boxes = document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');
  const matches = buildFilter(store);
  if (!matches) {
    for (const box of boxes) toggleElem(box, true);
    toggleElem('#diff-no-matches', false);
    return;
  }
  let visibleCount = 0;
  for (const box of boxes) {
    const newName = box.getAttribute('data-new-filename') ?? '';
    const oldName = box.getAttribute('data-old-filename') ?? '';
    const matched = matches(newName) || (oldName !== newName && matches(oldName));
    if (matched) visibleCount++;
    toggleElem(box, matched);
  }
  toggleElem('#diff-no-matches', visibleCount === 0);
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    return entry.Children.every((child) => child.IsViewed);
  }
  return entry.IsViewed;
}
