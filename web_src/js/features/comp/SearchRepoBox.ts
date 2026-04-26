import {chooseFromApi} from '../../modules/search.ts';

const {appSubUrl} = window.config;

type RepoSearchResponse = {data: Array<{repository: {full_name: string}}>};

export async function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const exclusive = el.getAttribute('data-exclusive');
  let url = `${appSubUrl}/repo/search?q={query}&uid=${uid}`;
  if (exclusive === 'true') url += `&exclusive=true`;
  const input = el.querySelector<HTMLInputElement>('input.prompt')!;

  while (el.isConnected) {
    const pick = await chooseFromApi<RepoSearchResponse>(el, url, (response) => response.data.map((item) => ({
      title: item.repository.full_name.split('/')[1],
      description: item.repository.full_name,
    })));
    input.value = pick.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
  }
}
