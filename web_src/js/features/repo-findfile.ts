import {createApp} from 'vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepoFileSearch() {
  registerGlobalInitFunc('initRepoFileSearch', async (el) => {
    const {default: RepoFileSearch} = await import('../components/RepoFileSearch.vue');
    createApp(RepoFileSearch, {
      repoLink: el.getAttribute('data-repo-link'),
      currentRefNameSubURL: el.getAttribute('data-current-ref-name-sub-url'),
      treeListUrl: el.getAttribute('data-tree-list-url'),
      noResultsText: el.getAttribute('data-no-results-text'),
      placeholder: el.getAttribute('data-placeholder'),
    }).mount(el);
  });
}
