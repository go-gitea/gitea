import '../../fomantic/build/fomantic.js';
import {initGiteaFomantic} from '../modules/fomantic.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {initRepoSwitcher} from './repo-switcher.ts';

test('repo-switcher opens and lists repositories', async () => {
  initGiteaFomantic();

  const el = createElementFromHTML(`<div class="ui search dropdown repo-switcher" data-uid="2" data-current-full-name="user2/repo1">
    <svg class="svg octicon-triangle-down dropdown icon"></svg>
    <input type="text" class="search" autocomplete="off">
    <div class="menu"></div>
  </div>`);
  document.body.append(el);

  let requestedUrl = '';
  ($ as any).ajax = (settings: any) => {
    requestedUrl = settings.url;
    const deferred = ($ as any).Deferred();
    deferred.resolve({data: [
      {repository: {full_name: 'user2/repo1', html_url: '/user2/repo1', private: false, fork: false}},
      {repository: {full_name: 'user2/repo2', html_url: '/user2/repo2', private: true, fork: false}},
    ]}, 'success', {});
    return deferred.promise();
  };

  initRepoSwitcher(el as HTMLElement);
  el.querySelector('.dropdown.icon')!.dispatchEvent(new MouseEvent('click', {bubbles: true}));
  await new Promise((resolve) => setTimeout(resolve, 500));

  expect(requestedUrl).toContain('/repo/search?q=&uid=2&priority_owner_id=2');
  const items = el.querySelectorAll('.menu > .item');
  expect(items.length).toEqual(2);
  expect(items[0].getAttribute('data-value')).toEqual('/user2/repo1');

  // closing must not commit the highlighted item, otherwise Escape navigates away
  expect($(el).dropdown('setting', 'forceSelection')).toEqual(false);
});
