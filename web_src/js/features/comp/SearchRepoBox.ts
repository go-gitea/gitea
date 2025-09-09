import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {htmlEscape} from '../../utils/html.ts';

const {appSubUrl} = window.config;

export function initCompSearchRepoBox() {
  // on the page "page-content organization teams" and "page-content package settings"
  const searchRepobox = document.querySelector('#search-repo-box');
  if (!searchRepobox) return;

  const uid = searchRepobox.getAttribute('data-uid');

  const $searchRepoBox = fomanticQuery('#search-repo-box');
  $searchRepoBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/repo/search?q={query}&uid=${uid}`,
      onResponse(response: any) {
        const items = [];
        for (const item of response.data) {
          items.push({
            title: htmlEscape(item.repository.full_name.split('/')[1]),
            description: htmlEscape(item.repository.full_name),
          });
        }
        return {results: items};
      },
    },
    searchFields: ['full_name'],
    showNoResults: false,
  });
}
