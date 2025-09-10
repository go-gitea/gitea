import {initCompSearchRepoBox} from './comp/SearchRepoBox.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initPackageSettings() {
  registerGlobalInitFunc('initSearchRepoBox', initCompSearchRepoBox);
}
