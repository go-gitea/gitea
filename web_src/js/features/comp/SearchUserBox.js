import $ from 'jquery';
import {htmlEscape} from 'escape-goat';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

export function initCompSearchUserBox() {
  const $searchUserBox = $('#search-user-box');
  const allowEmailInput = $searchUserBox.attr('data-allow-email') === 'true';
  const allowEmailDescription = $searchUserBox.attr('data-allow-email-description');
  $searchUserBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/user/search?active=1&q={query}`,
      onResponse(response) {
        const items = [];
        const searchQuery = $searchUserBox.find('input').val();
        const searchQueryUppercase = searchQuery.toUpperCase();
        $.each(response.data, (_i, item) => {
          let title = item.login;
          if (item.full_name && item.full_name.length > 0) {
            title += ` (${htmlEscape(item.full_name)})`;
          }
          const resultItem = {
            title,
            image: item.avatar_url
          };
          if (searchQueryUppercase === item.login.toUpperCase()) {
            items.unshift(resultItem);
          } else {
            items.push(resultItem);
          }
        });

        if (allowEmailInput && items.length === 0 && looksLikeEmailAddressCheck.test(searchQuery)) {
          const resultItem = {
            title: searchQuery,
            description: allowEmailDescription
          };
          items.push(resultItem);
        }

        return {results: items};
      }
    },
    searchFields: ['login', 'full_name'],
    showNoResults: false
  });
}
