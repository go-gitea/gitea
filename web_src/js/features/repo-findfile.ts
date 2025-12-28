import {createApp} from 'vue';
import RepoFileSearch from '../components/RepoFileSearch.vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';

const threshold = 50;

// return the case-insensitive sub-match result as an array:  [unmatched, matched, unmatched, matched, ...]
// res[even] is unmatched, res[odd] is matched, see unit tests for examples
// argument subLower must be a lower-cased string.
export function strSubMatch(full: string, subLower: string) {
  const res = [''];
  let i = 0, j = 0;
  const fullLower = full.toLowerCase();
  while (i < subLower.length && j < fullLower.length) {
    if (subLower[i] === fullLower[j]) {
      if (res.length % 2 !== 0) res.push('');
      res[res.length - 1] += full[j];
      j++;
      i++;
    } else {
      if (res.length % 2 === 0) res.push('');
      res[res.length - 1] += full[j];
      j++;
    }
  }
  if (i !== subLower.length) {
    // if the sub string doesn't match the full, only return the full as unmatched.
    return [full];
  }
  if (j < full.length) {
    // append remaining chars from full to result as unmatched
    if (res.length % 2 === 0) res.push('');
    res[res.length - 1] += full.substring(j);
  }
  return res;
}

export function calcMatchedWeight(matchResult: Array<any>) {
  let weight = 0;
  for (let i = 0; i < matchResult.length; i++) {
    if (i % 2 === 1) { // matches are on odd indices, see strSubMatch
      // use a function f(x+x) > f(x) + f(x) to make the longer matched string has higher weight.
      weight += matchResult[i].length * matchResult[i].length;
    }
  }
  return weight;
}

export function filterRepoFilesWeighted(files: Array<string>, filter: string) {
  let filterResult = [];
  if (filter) {
    const filterLower = filter.toLowerCase();
    // TODO: for large repo, this loop could be slow, maybe there could be one more limit:
    // ... && filterResult.length < threshold * 20,  wait for more feedbacks
    for (const file of files) {
      const res = strSubMatch(file, filterLower);
      if (res.length > 1) { // length==1 means unmatched, >1 means having matched sub strings
        filterResult.push({matchResult: res, matchWeight: calcMatchedWeight(res)});
      }
    }
    filterResult.sort((a, b) => b.matchWeight - a.matchWeight);
    filterResult = filterResult.slice(0, threshold);
  } else {
    for (let i = 0; i < files.length && i < threshold; i++) {
      filterResult.push({matchResult: [files[i]], matchWeight: 0});
    }
  }
  return filterResult;
}

export function initRepoFileSearch() {
  registerGlobalInitFunc('initRepoFileSearch', (el) => {
    createApp(RepoFileSearch, {
      repoLink: el.getAttribute('data-repo-link'),
      currentRefNameSubURL: el.getAttribute('data-current-ref-name-sub-url'),
      treeListUrl: el.getAttribute('data-tree-list-url'),
      noResultsText: el.getAttribute('data-no-results-text'),
      placeholder: el.getAttribute('data-placeholder'),
    }).mount(el);
  });
}
