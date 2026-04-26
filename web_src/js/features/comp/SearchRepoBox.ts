import {chooseFromApi} from '../../modules/search.ts';

const {appSubUrl} = window.config;

export async function initCompSearchRepoBox(el: HTMLElement) {
  const uid = el.getAttribute('data-uid');
  const exclusive = el.getAttribute('data-exclusive');
  let url = `${appSubUrl}/repo/search?q={query}&uid=${uid}`;
  if (exclusive === 'true') url += `&exclusive=true`;
  const input = el.querySelector<HTMLInputElement>('input.prompt')!;

  while (el.isConnected) {
    const pick = await chooseFromApi(el, url, (response: any) => response.data.map((item: any) => ({
      title: item.repository.full_name.split('/')[1],
      description: item.repository.full_name,
    })));
    input.value = pick.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
  }
}
