import {initSearchBox} from '../../modules/fomantic/search.ts';
import {htmlEscape} from '../../utils/html.ts';

const {appSubUrl} = window.config;

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const exclusive = el.getAttribute('data-exclusive');
  let url = `${appSubUrl}/repo/search?q={query}&uid=${uid}`;
  if (exclusive === 'true') {
    url += `&exclusive=true`;
  }
  initSearchBox(el, {
    apiUrl: url,
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
  });
}
