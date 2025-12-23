import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {html} from '../../utils/html.ts';

const {appSubUrl} = window.config;

export function initCompSearchTeamBox() {
  const searchTeamBox = document.querySelector('#search-team-box');
  if (!searchTeamBox) return;

  fomanticQuery(searchTeamBox).search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}${searchTeamBox.getAttribute('data-search-url')}`,
      onResponse(response: {data: any[]}) {
        const resultItems = [];
        const searchQuery = searchTeamBox.querySelector('input').value;
        const searchQueryUppercase = searchQuery.toUpperCase();
        for (const item of response.data) {
          const resultItem = {
            title: item.name,
            description: html`${item.description}`,
          };
          if (searchQueryUppercase === item.name.toUpperCase()) {
            resultItems.unshift(resultItem); // add the exact match to the top
          } else {
            resultItems.push(resultItem);
          }
        }

        return {results: resultItems};
      },
    },
    searchFields: ['login', 'full_name'],
    showNoResults: false,
  });
}
