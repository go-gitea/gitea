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

type DiffFileTree = {
  folderIcon: string;
  folderOpenIcon: string;
  diffFileTree: DiffFileTreeData;
  fullNameMap: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
  filenameFilterQuery: string;
  activeExtensions: string[] | null;
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
  const store = reactive({
    diffFileTree: data,
    folderIcon,
    folderOpenIcon,
    fileTreeIsVisible: false,
    selectedItem: '',
    filenameFilterQuery: '',
    activeExtensions: null,
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

export function getDiffTreeExtensionStats(store: Reactive<DiffFileTree>): DiffExtensionStats[] {
  const extensionMap = new Map<string, number>();
  for (const entry of Object.values(store.fullNameMap)) {
    if (entry.EntryMode === 'tree' || !entry.FullName) continue;
    const ext = extname(entry.FullName);
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }
  return Array.from(extensionMap.entries(), ([ext, count]) => ({ext, count}))
    .sort((a, b) => b.count - a.count);
}

export type FileFilterPredicate = (filename: string) => boolean;

export function buildFilterPredicate(store: Reactive<DiffFileTree>): FileFilterPredicate {
  const query = store.filenameFilterQuery.trim().toLowerCase();
  const activeExtSet = store.activeExtensions ? new Set(store.activeExtensions) : null;
  return (filename) => {
    if (query && !filename.toLowerCase().includes(query)) return false;
    if (activeExtSet && !activeExtSet.has(extname(filename))) return false;
    return true;
  };
}

export function isDiffTreeEntryVisible(entry: DiffTreeEntry, matches: FileFilterPredicate): boolean {
  if (entry.EntryMode === 'tree') {
    return Boolean(entry.Children?.some((child) => isDiffTreeEntryVisible(child, matches)));
  }
  return matches(entry.FullName);
}

export function applyFiltersToFileBoxes(store: Reactive<DiffFileTree>) {
  const matches = buildFilterPredicate(store);
  const isFiltering = Boolean(store.filenameFilterQuery.trim()) || store.activeExtensions !== null;
  let visibleCount = 0;
  for (const box of document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]')) {
    const matched = matches(box.getAttribute('data-new-filename') ?? '');
    if (matched) visibleCount++;
    toggleElem(box, matched);
  }
  const empty = document.querySelector('#diff-no-matches');
  if (empty) toggleElem(empty, visibleCount === 0 && isFiltering);
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    return entry.Children.every((child) => child.IsViewed);
  }
  return entry.IsViewed;
}
