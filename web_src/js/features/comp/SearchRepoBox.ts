import {fomanticQuery} from '../../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

export function initCompSearchRepoBox() {
  // on the page "page-content organization teams"
  const $searchRepoBox = fomanticQuery('#search-repo-box');
  const showFullName = $searchRepoBox.data('full-name');
  $searchRepoBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/repo/search?q={query}&uid=${$searchRepoBox.data('uid')}`,
      onResponse(response: any) {
        const items = [];
        for (const item of response.data) {
          let title = item.repository.full_name.split('/')[1];
          if (showFullName) {
            title = item.repository.full_name;
          }
          items.push({
            title,
            description: item.repository.full_name,
          });
        }
        return {results: items};
      },
    },
    searchFields: ['full_name'],
    showNoResults: false,
  });
}
