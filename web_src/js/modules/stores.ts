import {reactive} from 'vue';
import type {Reactive} from 'vue';

let diffTreeStoreReactive: Reactive<Record<string, any>>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactive(window.config.pageData.diffFileInfo);
    window.config.pageData.diffFileInfo = diffTreeStoreReactive;
  }
  return diffTreeStoreReactive;
}

let viewTreeStoreReactive: Reactive<Record<string, any>>;
export function viewTreeStore() {
  if (!viewTreeStoreReactive) {
    viewTreeStoreReactive = reactive(window.config.pageData.viewFileInfo);
    window.config.pageData.viewFileInfo = viewTreeStoreReactive;
  }
  return viewTreeStoreReactive;
}
