import {attachSearchBox} from '../../modules/search.ts';

const {appSubUrl} = window.config;

type RepoSearchResponse = {data: Array<{repository: {full_name: string}}>};

export function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const exclusive = el.getAttribute('data-exclusive');
  let url = `${appSubUrl}/repo/search?q={query}&uid=${uid}`;
  if (exclusive === 'true') url += `&exclusive=true`;
  attachSearchBox(el, url, (response: RepoSearchResponse) => response.data.map((item) => ({
    title: item.repository.full_name.split('/')[1],
    description: item.repository.full_name,
  })));
}
