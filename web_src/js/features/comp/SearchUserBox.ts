import {htmlEscape} from 'escape-goat';
import {fomanticQuery} from '../../modules/fomantic/base.ts';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

export function initCompSearchUserBox() {
  const searchUserBox = document.querySelector('#search-user-box');
  if (!searchUserBox) return;

  const allowEmailInput = searchUserBox.getAttribute('data-allow-email') === 'true';
  const allowEmailDescription = searchUserBox.getAttribute('data-allow-email-description') ?? undefined;
  fomanticQuery(searchUserBox).search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/user/search_candidates?q={query}`,
      onResponse(response) {
        const resultItems = [];
        const searchQuery = searchUserBox.querySelector('input').value;
        const searchQueryUppercase = searchQuery.toUpperCase();
        for (const item of response.data) {
          const resultItem = {
            title: item.login,
            image: item.avatar_url,
            description: htmlEscape(item.full_name),
          };
          if (searchQueryUppercase === item.login.toUpperCase()) {
            resultItems.unshift(resultItem); // add the exact match to the top
          } else {
            resultItems.push(resultItem);
          }
        }

        if (allowEmailInput && !resultItems.length && looksLikeEmailAddressCheck.test(searchQuery)) {
          const resultItem = {
            title: searchQuery,
            description: allowEmailDescription,
          };
          resultItems.push(resultItem);
        }

        return {results: resultItems};
      },
    },
    searchFields: ['login', 'full_name'],
    showNoResults: false,
  });
}
