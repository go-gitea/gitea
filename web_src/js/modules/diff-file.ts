import {reactive} from 'vue';
import type {Reactive} from 'vue';

const {pageData} = window.config;

export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechange';

export type DiffTreeEntry = {
  FullName: string,
  DisplayName: string,
  NameHash: string,
  DiffStatus: DiffStatus,
  EntryMode: string,
  IsViewed: boolean,
  Children: DiffTreeEntry[],

  ParentEntry?: DiffTreeEntry,
}

type DiffFileTreeData = {
  TreeRoot: DiffTreeEntry,
};

type DiffFileTree = {
  diffFileTree: DiffFileTreeData;
  fullNameMap?: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
}

let diffTreeStoreReactive: Reactive<DiffFileTree>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactiveDiffTreeStore(pageData.DiffFileTree);
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

export function reactiveDiffTreeStore(data: DiffFileTreeData): Reactive<DiffFileTree> {
  const store = reactive({
    diffFileTree: data,
    fileTreeIsVisible: false,
    selectedItem: '',
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    let count = 0;
    for (const child of entry.Children) {
      if (child.IsViewed) count++;
    }
    return count === entry.Children.length;
  }
  return entry.IsViewed;
}
