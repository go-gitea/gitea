import {initDropdown} from '../../modules/dropdown.ts';
import {createElementFromAttrs} from '../../utils/dom.ts';
import {fetchSearchResults} from './SearchUserBox.ts';

type RepoSearchResult = {repository: {full_name: string}};

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid')!;
  const exclusive = Number(uid) > 0;
  initDropdown(el, {
    async loadItems(query, signal) {
      const params: Record<string, string> = {q: query, uid};
      if (exclusive) params.exclusive = 'true';
      const data = await fetchSearchResults<RepoSearchResult>('/repo/search', params, signal);
      return data?.map(({repository: {full_name}}) => createElementFromAttrs('div', {
        class: 'dropdown-item', 'data-value': exclusive ? full_name.split('/')[1] : full_name,
      }, full_name)) ?? [];
    },
  });
}
