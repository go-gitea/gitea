import {htmlEscape} from '../../utils/html.ts';
import {initSearchBox} from '../../modules/fomantic/search.ts';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

export function initCompSearchUserBox() {
  const searchUserBox = document.querySelector<HTMLElement>('#search-user-box');
  if (!searchUserBox) return;

  const allowEmailInput = searchUserBox.getAttribute('data-allow-email') === 'true';
  const allowEmailDescription = searchUserBox.getAttribute('data-allow-email-description') ?? undefined;
  const includeOrgs = searchUserBox.getAttribute('data-include-orgs') === 'true';
  initSearchBox(searchUserBox, {
    apiUrl: `${appSubUrl}/user/search_candidates?q={query}&orgs=${includeOrgs}`,
    onResponse(response: any, searchQuery: string) {
      const resultItems = [];
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
  });
}
