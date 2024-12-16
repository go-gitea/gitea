import {createApp, ref} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {GET, PUT} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';

async function toggleSidebar(visibility) {
  const sidebarEl = document.querySelector('.repo-view-file-tree-sidebar');
  const showBtnEl = document.querySelector('.show-tree-sidebar-button');
  const refSelectorEl = document.querySelector('.repo-home-filelist .js-branch-tag-selector');
  const newPrBtnEl = document.querySelector('.repo-home-filelist #new-pull-request');
  const addFileEl = document.querySelector('.repo-home-filelist .add-file-dropdown');
  const containerClassList = sidebarEl.parentElement.classList;
  containerClassList.toggle('repo-grid-tree-sidebar', visibility);
  containerClassList.toggle('repo-grid-filelist-only', !visibility);
  toggleElem(sidebarEl, visibility);
  toggleElem(showBtnEl, !visibility);
  toggleElem(refSelectorEl, !visibility);
  toggleElem(newPrBtnEl, !visibility);
  if (addFileEl) {
    toggleElem(addFileEl, !visibility);
  }

  // save to session
  await PUT('/repo/preferences', {
    data: {
      show_file_view_tree_sidebar: visibility,
    },
  });
}

async function loadChildren(item?) {
  const el = document.querySelector('#view-file-tree');
  const apiBaseUrl = el.getAttribute('data-api-base-url');
  const response = await GET(`${apiBaseUrl}/contents/${item ? item.path : ''}`);
  const json = await response.json();
  if (json instanceof Array) {
    return json.map((i) => ({
      name: i.name,
      isFile: i.type === 'file',
      htmlUrl: i.html_url,
      path: i.path,
    }));
  }
  return null;
}

async function loadRecursive(treePath) {
  let root = null;
  let parent = null;
  let parentPath = '';
  for (const i of (`/${treePath}`).split('/')) {
    const path = `${parentPath}${parentPath ? '/' : ''}${i}`;
    const result = await loadChildren({path});
    if (root === null) {
      root = result;
      parent = root;
    } else {
      parent = parent.find((item) => item.path === path);
      parent.children = result;
      parent = result;
    }
    parentPath = path;
  }
  return root;
}

async function loadContent(item) {
  document.querySelector('.repo-home-filelist').innerHTML = `load content of ${item.path}`;
}

export async function initViewFileTreeSidebar() {
  const sidebarElement = document.querySelector('.repo-view-file-tree-sidebar');
  if (!sidebarElement) return;

  document.querySelector('.show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(true);
  });

  document.querySelector('.hide-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(false);
  });

  const fileTree = document.querySelector('#view-file-tree');
  const treePath = fileTree.getAttribute('data-tree-path');
  const selectedItem = ref(treePath);

  const files = await loadRecursive(treePath);

  fileTree.classList.remove('center');
  const fileTreeView = createApp(ViewFileTree, {files, selectedItem, loadChildren, loadContent: (item) => {
    window.history.pushState(null, null, item.htmlUrl);
    selectedItem.value = item.path;
    loadContent(item);
  }});
  fileTreeView.mount(fileTree);
}
