import {createApp} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';

const {appSubUrl} = window.config;

async function toggleSidebar(sidebarEl: HTMLElement, shouldShow: boolean) {
  const showBtnEl = sidebarEl.parentElement.querySelector('.show-tree-sidebar-button');
  const containerClassList = sidebarEl.parentElement.classList;
  containerClassList.toggle('repo-view-with-sidebar', shouldShow);
  containerClassList.toggle('repo-view-content-only', !shouldShow);
  toggleElem(sidebarEl, shouldShow);
  toggleElem(showBtnEl, !shouldShow);

  // FIXME: need to remove "full height" style from parent element

  if (!sidebarEl.hasAttribute('data-is-signed')) return;

  // save to session
  await POST(`${appSubUrl}/user/settings/update_preferences`, {
    data: {
      codeViewShowFileTree: shouldShow,
    },
  });

  // FIXME: add event listener for "show-tree-sidebar-button"
}

export async function initViewFileTreeSidebar() {
  const sidebar = document.querySelector<HTMLElement>('.repo-view-file-tree-sidebar');
  const repoViewContent = document.querySelector('.repo-view-content');
  if (!sidebar || !repoViewContent) return;

  sidebar.querySelector('.hide-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(sidebar, false);
  });
  repoViewContent.querySelector('.show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(sidebar, true);
  });

  const fileTree = sidebar.querySelector('#view-file-tree');
  createApp(ViewFileTree, {
    repoLink: fileTree.getAttribute('data-repo-link'),
    treePath: fileTree.getAttribute('data-tree-path'),
    currentRefNameSubURL: fileTree.getAttribute('data-current-ref-name-sub-url'),
  }).mount(fileTree);
}
