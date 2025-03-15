import {createApp, ref} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {pathEscapeSegments, pathUnescapeSegments} from '../utils/url.ts';
import {GET, POST} from '../modules/fetch.ts';
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
}

function childrenLoader(sidebarEl: HTMLElement) {
  return async (path: string, recursive?: boolean) => {
    const fileTree = sidebarEl.querySelector('#view-file-tree');
    const apiBaseUrl = fileTree.getAttribute('data-api-base-url');
    const refTypeNameSubURL = fileTree.getAttribute('data-current-ref-type-name-sub-url');
    const response = await GET(`${apiBaseUrl}/tree/${refTypeNameSubURL}/${pathEscapeSegments(path ?? '')}?recursive=${recursive ?? false}`);
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
  };
}

async function loadContent(sidebarEl: HTMLElement) {
  // load content by path (content based on home_content.tmpl)
  const response = await GET(`${window.location.href}?only_content=true`);
  const contentEl = sidebarEl.parentElement.querySelector('.repo-view-content');
  contentEl.innerHTML = await response.text();
  reloadContentScript(sidebarEl, contentEl);
}

function reloadContentScript(sidebarEl: HTMLElement, contentEl: Element) {
  contentEl.querySelector('.show-tree-sidebar-button')?.addEventListener('click', () => {
    toggleSidebar(sidebarEl, true);
  });
}

export async function initViewFileTreeSidebar() {
  const sidebarEl = document.querySelector('.repo-view-file-tree-sidebar');
  if (!sidebarEl || !(sidebarEl instanceof HTMLElement)) return;

  sidebarEl.querySelector('.hide-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(sidebarEl, false);
  });
  sidebarEl.parentElement.querySelector('.repo-view-content .show-tree-sidebar-button').addEventListener('click', () => {
    toggleSidebar(sidebarEl, true);
  });

  const fileTree = sidebarEl.querySelector('#view-file-tree');
  const baseUrl = fileTree.getAttribute('data-api-base-url');
  const treePath = fileTree.getAttribute('data-tree-path');
  const refType = fileTree.getAttribute('data-current-ref-type');
  const refName = fileTree.getAttribute('data-current-ref-short-name');
  const refString = (refType ? (`/${refType}`) : '') + (refName ? (`/${refName}`) : '');

  const selectedItem = ref(getSelectedPath(refString));

  const files = await childrenLoader(sidebarEl)(treePath, true);

  fileTree.classList.remove('is-loading');
  const fileTreeView = createApp(ViewFileTree, {files, selectedItem, loadChildren: childrenLoader(sidebarEl), loadContent: (path: string) => {
    window.history.pushState(null, null, `${baseUrl}/src${refString}/${pathEscapeSegments(path)}`);
    selectedItem.value = path;
    loadContent(sidebarEl);
  }});
  fileTreeView.mount(fileTree);

  window.addEventListener('popstate', () => {
    selectedItem.value = getSelectedPath(refString);
    loadContent(sidebarEl);
  });
}

function getSelectedPath(ref: string) {
  const path = pathUnescapeSegments(new URL(window.location.href).pathname);
  return path.substring(path.indexOf(ref) + ref.length + 1);
}
