import {chooseFromApi, type SearchResult} from '../../modules/search.ts';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

export async function initCompSearchUserBox() {
  const box = document.querySelector<HTMLElement>('#search-user-box');
  if (!box) return;

  const allowEmailInput = box.getAttribute('data-allow-email') === 'true';
  const allowEmailDescription = box.getAttribute('data-allow-email-description') ?? undefined;
  const includeOrgs = box.getAttribute('data-include-orgs') === 'true';
  const url = `${appSubUrl}/user/search_candidates?q={query}&orgs=${includeOrgs}`;
  const input = box.querySelector<HTMLInputElement>('input.prompt')!;

  while (box.isConnected) {
    const pick = await chooseFromApi(box, url, (response: any, query: string) => {
      const items: SearchResult[] = [];
      const queryUpper = query.toUpperCase();
      for (const item of response.data) {
        const result: SearchResult = {title: item.login, image: item.avatar_url, description: item.full_name};
        if (queryUpper === item.login.toUpperCase()) items.unshift(result); // exact match floats to top
        else items.push(result);
      }
      if (allowEmailInput && !items.length && looksLikeEmailAddressCheck.test(query)) {
        items.push({title: query, description: allowEmailDescription});
      }
      return items;
    });
    input.value = pick.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
  }
}
