import {reactive} from 'vue';
import type {Reactive} from 'vue';

const {pageData} = window.config;

let diffTreeStoreReactive: Reactive<Record<string, any>>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactive({
      files: pageData.DiffFiles,
      fileTreeIsVisible: false,
      selectedItem: '',
    });
  }
  return diffTreeStoreReactive;
}
