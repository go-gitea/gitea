import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {htmlEscape} from '../../utils/html.ts';

const {appSubUrl} = window.config;

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  fomanticQuery(el).search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/repo/search?q={query}&uid=${uid}`,
      onResponse(response: any) {
        const items = [];
        for (const item of response.data) {
          // Show repository full_name as title for clear identification
          // Show subject as description to provide context about the repository
          const title = htmlEscape(item.repository.full_name);
          const description = item.repository.subject ?
            htmlEscape(item.repository.subject) :
            '';

          items.push({
            title,
            description,
          });
        }
        return {results: items};
      },
    },
    searchFields: ['full_name', 'subject'],
    showNoResults: false,
  });
}
