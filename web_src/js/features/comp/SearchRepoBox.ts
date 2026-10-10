import {attachSearchBox} from '../../modules/search.ts';

const {appSubUrl} = window.config;

export type RepoSearchResponse = {data: Array<{repository: {id: number; full_name: string}}>};

export function repoSearchUrl(uid: string, exclusive: boolean): string {
  return `${appSubUrl}/repo/search?q={query}&uid=${uid}${exclusive ? '&exclusive=true' : ''}`;
}

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const exclusive = el.getAttribute('data-exclusive');
  // when set, the selected value is the full "owner/name" rather than the bare repo name, so a cross-owner search can be resolved unambiguously
  const fullName = el.getAttribute('data-full-name') === 'true';
  attachSearchBox(el, repoSearchUrl(uid!, exclusive === 'true'), (response: RepoSearchResponse) => response.data.map((item) => ({
    title: fullName ? item.repository.full_name : item.repository.full_name.split('/')[1],
    description: item.repository.full_name,
  })));
}
