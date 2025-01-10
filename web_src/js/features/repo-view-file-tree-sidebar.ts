import {createApp, ref} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {GET, PUT} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';
import RepoBranchTagSelector from '../components/RepoBranchTagSelector.vue';
import {initGlobalDropdown} from './common-page.ts';
import {initRepoEllipsisButton} from './repo-commit.ts';
import {initPdfViewer} from '../render/pdf.ts';
import {initGlobalButtons} from './common-button.ts';

async function toggleSidebar(visibility, isSigned) {
  const sidebarEl = document.querySelector('.repo-view-file-tree-sidebar');
  const showBtnEl = document.querySelector('.show-tree-sidebar-button');
  const containerClassList = sidebarEl.parentElement.classList;
  containerClassList.toggle('repo-grid-tree-sidebar', visibility);
  containerClassList.toggle('repo-grid-filelist-only', !visibility);
  toggleElem(sidebarEl, visibility);
  toggleElem(showBtnEl, !visibility);

  if (!isSigned) return;

  // save to session
  await PUT('/repo/preferences', {
    data: {
      show_file_view_tree_sidebar: visibility,
    },
  });
}

async function loadChildren(item, recursive?: boolean) {
  const fileTree = document.querySelector('#view-file-tree');
  const apiBaseUrl = fileTree.getAttribute('data-api-base-url');
  const refType = fileTree.getAttribute('data-current-ref-type');
  const refName = fileTree.getAttribute('data-current-ref-short-name');
  const response = await GET(`${apiBaseUrl}/tree/${item ? item.path : ''}?ref=${refType}/${refName}&recursive=${recursive ?? false}`);
  const json = await response.json();
  if (json instanceof Array) {
    return json.map((i) => ({
      name: i.name,
      isFile: i.isFile,
      path: i.path,
      children: i.children,
    }));
  }
  return null;
}

async function loadContent() {
  // load content by path (content based on home_content.tmpl)
  const response = await GET(`${window.location.href}?only_content=true`);
  document.querySelector('.repo-home-filelist').innerHTML = await response.text();
  reloadContentScript();
}

function reloadContentScript() {
  document.querySelector('.repo-home-filelist .show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(true, document.querySelector('.repo-view-file-tree-sidebar').hasAttribute('data-is-signed'));
  });
  const refSelectorEl = document.querySelector('.repo-home-filelist .js-branch-tag-selector');
  if (refSelectorEl) {
    createApp(RepoBranchTagSelector, {elRoot: refSelectorEl}).mount(refSelectorEl);
  }
  initGlobalDropdown();
  initRepoEllipsisButton();
  initPdfViewer();
  initGlobalButtons();
}

export async function initViewFileTreeSidebar() {
  const sidebarElement = document.querySelector('.repo-view-file-tree-sidebar');
  if (!sidebarElement) return;

  const isSigned = sidebarElement.hasAttribute('data-is-signed');

  document.querySelector('.hide-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(false, isSigned);
  });
  document.querySelector('.repo-home-filelist .show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(true, isSigned);
  });

  const fileTree = document.querySelector('#view-file-tree');
  const baseUrl = fileTree.getAttribute('data-api-base-url');
  const treePath = fileTree.getAttribute('data-tree-path');
  const refType = fileTree.getAttribute('data-current-ref-type');
  const refName = fileTree.getAttribute('data-current-ref-short-name');

  const selectedItem = ref(treePath);

  const files = await loadChildren({path: treePath}, true);

  fileTree.classList.remove('is-loading');
  const fileTreeView = createApp(ViewFileTree, {files, selectedItem, loadChildren, loadContent: (item) => {
    window.history.pushState(null, null, `${baseUrl}/src/${refType}/${refName}/${item.path}`);
    selectedItem.value = item.path;
    loadContent();
  }});
  fileTreeView.mount(fileTree);
}
