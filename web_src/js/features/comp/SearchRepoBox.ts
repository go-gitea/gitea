import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {htmlEscape} from '../../utils/html.ts';

const {appSubUrl} = window.config;

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const collaborate = el.getAttribute('data-collaborate');
  let url = `${appSubUrl}/repo/search?q={query}&uid=${uid}`;
  if (collaborate !== null) {
    url += `&collaborate=${collaborate}`;
  }
  fomanticQuery(el).search({
    minCharacters: 2,
    apiSettings: {
      url,
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
