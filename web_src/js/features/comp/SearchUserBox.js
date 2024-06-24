import $ from 'jquery';
import {htmlEscape} from 'escape-goat';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

export function initCompSearchUserBox() {
  const searchUserBox = document.querySelector('#search-user-box');
  if (!searchUserBox) return;

  const $searchUserBox = $(searchUserBox);
  const allowEmailInput = searchUserBox.getAttribute('data-allow-email') === 'true';
  const allowEmailDescription = searchUserBox.getAttribute('data-allow-email-description') ?? undefined;
  $searchUserBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/user/search?active=1&q={query}`,
      onResponse(response) {
        const items = [];
        const searchQuery = $searchUserBox.find('input').val();
        const searchQueryUppercase = searchQuery.toUpperCase();
        $.each(response.data, (_i, item) => {
          const resultItem = {
            title: item.login,
            image: item.avatar_url,
          };
          if (item.full_name) {
            resultItem.description = htmlEscape(item.full_name);
          }
          if (searchQueryUppercase === item.login.toUpperCase()) {
            items.unshift(resultItem);
          } else {
            items.push(resultItem);
          }
        });

        if (allowEmailInput && !items.length && looksLikeEmailAddressCheck.test(searchQuery)) {
          const resultItem = {
            title: searchQuery,
            description: allowEmailDescription,
          };
          items.push(resultItem);
        }

        return {results: items};
      },
    },
    searchFields: ['login', 'full_name'],
    showNoResults: false,
  });
}
