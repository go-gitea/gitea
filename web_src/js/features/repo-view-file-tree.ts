import {createApp} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';
import {registerGlobalEventFunc} from '../modules/observer.ts';

const {appSubUrl} = window.config;

async function toggleSidebar(btn: HTMLElement) {
  const elToggleShow = document.querySelector('.repo-view-file-tree-toggle-show');
  const elFileTreeContainer = document.querySelector('.repo-view-file-tree-container');
  const shouldShow = btn.getAttribute('data-toggle-action') === 'show';
  toggleElem(elFileTreeContainer, shouldShow);
  toggleElem(elToggleShow, !shouldShow);

  // FIXME: need to remove "full height" style from parent element

  if (!elFileTreeContainer.hasAttribute('data-user-is-signed-in')) return;
  await POST(`${appSubUrl}/user/settings/update_preferences`, {
    data: {codeViewShowFileTree: shouldShow},
  });
}

export async function initRepoViewFileTree() {
  const sidebar = document.querySelector<HTMLElement>('.repo-view-file-tree-container');
  const repoViewContent = document.querySelector('.repo-view-content');
  if (!sidebar || !repoViewContent) return;

  registerGlobalEventFunc('click', 'onRepoViewFileTreeToggle', toggleSidebar);

  const fileTree = sidebar.querySelector('#view-file-tree');
  createApp(ViewFileTree, {
    repoLink: fileTree.getAttribute('data-repo-link'),
    treePath: fileTree.getAttribute('data-tree-path'),
    currentRefNameSubURL: fileTree.getAttribute('data-current-ref-name-sub-url'),
  }).mount(fileTree);
}
