import {createApp, ref} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {GET, PUT} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';
import {initTargetRepoBranchTagSelector} from './repo-legacy.ts';
import {initTargetDropdown} from './common-page.ts';
import {initTargetRepoEllipsisButton} from './repo-commit.ts';
import {initTargetPdfViewer} from '../render/pdf.ts';
import {initTargetButtons} from './common-button.ts';
import {initTargetCopyContent} from './copycontent.ts';

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
  const refTypeNameSubURL = fileTree.getAttribute('data-current-ref-type-name-sub-url');
  const response = await GET(`${apiBaseUrl}/tree/${refTypeNameSubURL}/${item ? item.path : ''}?recursive=${recursive ?? false}`);
  const json = await response.json();
  if (json instanceof Array) {
    return json.map((i) => ({
      name: i.name,
      type: i.type,
      path: i.path,
      sub_module_url: i.sub_module_url,
      children: i.children,
    }));
  }
  return null;
}

async function loadContent() {
  // load content by path (content based on home_content.tmpl)
  const response = await GET(`${window.location.href}?only_content=true`);
  const contentEl = document.querySelector('.repo-home-filelist');
  contentEl.innerHTML = await response.text();
  reloadContentScript(contentEl);
}

function reloadContentScript(contentEl: Element) {
  contentEl.querySelector('.show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(true, document.querySelector('.repo-view-file-tree-sidebar').hasAttribute('data-is-signed'));
  });
  initTargetButtons(contentEl);
  initTargetDropdown(contentEl);
  initTargetPdfViewer(contentEl);
  initTargetRepoBranchTagSelector(contentEl);
  initTargetRepoEllipsisButton(contentEl);
  initTargetCopyContent(contentEl);
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
  const refString = (refType ? (`/${refType}`) : '') + (refName ? (`/${refName}`) : '');

  const selectedItem = ref(treePath);

  const files = await loadChildren({path: treePath}, true);

  fileTree.classList.remove('is-loading');
  const fileTreeView = createApp(ViewFileTree, {files, selectedItem, loadChildren, loadContent: (item) => {
    window.history.pushState(null, null, `${baseUrl}/src${refString}/${item.path}`);
    selectedItem.value = item.path;
    loadContent();
  }});
  fileTreeView.mount(fileTree);

  window.addEventListener('popstate', () => {
    selectedItem.value = extractPath(window.location.href);
    loadContent();
  });
}

function extractPath(url) {
  // Create a URL object
  const urlObj = new URL(url);

  // Get the pathname part
  const path = urlObj.pathname;

  // Define a regular expression to match "/{param1}/{param2}/src/{branch}/{main}/"
  const regex = /^\/[^/]+\/[^/]+\/src\/[^/]+\/[^/]+\//;

  // Use RegExp#exec() method to match the path
  const match = regex.exec(path);
  if (match) {
    return path.substring(match[0].length);
  }

  // If the path does not match, return the original path
  return path;
}
