import {initDropdown} from '../../modules/dropdown.ts';
import {GET} from '../../modules/fetch.ts';
import {createElementFromAttrs} from '../../utils/dom.ts';

const {appSubUrl} = window.config;
const looksLikeEmailAddressCheck = /^\S+@\S+$/;

type UserSearchResult = {login: string; avatar_url: string; full_name: string};

export async function fetchSearchResults<T>(endpoint: string, params: Record<string, string>, signal: AbortSignal): Promise<T[] | null> {
  const response = await GET(`${appSubUrl}${endpoint}?${new URLSearchParams(params)}`, {signal});
  return response.ok ? (await response.json() as {data: T[]}).data : null;
}

function userItem(login: string, fullName: string | null, avatarUrl?: string) {
  const item = createElementFromAttrs('div', {class: 'dropdown-item flex-text-block', 'data-value': login});
  if (avatarUrl) item.append(createElementFromAttrs('img', {class: 'ui avatar', src: avatarUrl, width: 28, height: 28, alt: ''}));
  const text = createElementFromAttrs('div', null, login);
  if (fullName) text.append(createElementFromAttrs('span', {class: 'dropdown-item-detail'}, fullName));
  item.append(text);
  return item;
}

export function initCompSearchUserBox(box: HTMLElement) {
  const emailDescription = box.getAttribute('data-allow-email-description');
  const includeOrgs = box.getAttribute('data-include-orgs') === 'true';

  initDropdown(box, {
    async loadItems(query, signal) {
      const data = await fetchSearchResults<UserSearchResult>('/user/search_candidates', {q: query, orgs: String(includeOrgs)}, signal);
      if (!data) return [];
      if (emailDescription !== null && !data.length && looksLikeEmailAddressCheck.test(query)) return [userItem(query, emailDescription)];

      const items: HTMLElement[] = [];
      const queryUpper = query.toUpperCase();
      for (const {login, full_name, avatar_url} of data) {
        const item = userItem(login, full_name, avatar_url);
        if (queryUpper === login.toUpperCase()) items.unshift(item);
        else items.push(item);
      }
      return items;
    },
  });
}
