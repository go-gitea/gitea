import {initCompSearchRepoBox} from './comp/SearchRepoBox.ts';

export function initPackageSettings() {
  if (!document.querySelector('.page-content.package.settings')) return;
  initCompSearchRepoBox();
}
