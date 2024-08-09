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
