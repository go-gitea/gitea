import {createApp, nextTick} from 'vue';
import RepoBranchTagSelector from './RepoBranchTagSelector.vue';

test('writes a selected branch to a form input', async () => {
  document.body.innerHTML = `
    <input id="source-branch" value="main">
    <div id="branch-selector"
      data-current-repo-default-branch="main"
      data-current-repo-link="/owner/repo"
      data-current-tree-path=""
      data-current-ref-type="branch"
      data-current-ref-short-name="main"
      data-ref-link-template=""
      data-ref-form-action-template=""
      data-ref-input-id="source-branch"
      data-dropdown-fixed-text=""
      data-show-tab-branches="true"
      data-show-tab-tags="false"
      data-allow-create-new-ref="false"
      data-show-view-all-refs-entry="false"
      data-enable-feed="false"
      data-text-branches="Branches"
      data-text-tags="Tags"
      data-text-filter-branch="Filter branches"
      data-text-filter-tag="Filter tags"
      data-text-default-branch-label="default"
      data-text-create-tag="Create tag %s"
      data-text-create-branch="Create branch %s"
      data-text-create-ref-from="from %s"
      data-text-no-results="No results"
      data-text-view-all-branches="View all branches"
      data-text-view-all-tags="View all tags"
    ></div>`;
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json({results: ['main', 'pages']}));

  const root = document.querySelector<HTMLElement>('#branch-selector')!;
  const app = createApp(RepoBranchTagSelector, {elRoot: root});
  app.mount(root);
  await nextTick();
  root.querySelector<HTMLButtonElement>('.branch-dropdown-button')!.click();
  await expect.poll(() => root.querySelectorAll('.scrolling.menu .item').length).toBe(2);
  root.querySelectorAll<HTMLElement>('.scrolling.menu .item')[1].click();
  await nextTick();

  expect(document.querySelector<HTMLInputElement>('#source-branch')!.value).toBe('pages');
  expect(root.querySelector('.branch-dropdown-button')!.textContent).toContain('pages');

  app.unmount();
  vi.restoreAllMocks();
});
