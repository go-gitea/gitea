import {createApp, ref} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {GET, PUT} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import ViewFileTree from '../components/ViewFileTree.vue';

async function toggleSidebar(sidebarEl: HTMLElement, visibility: boolean) {
  const showBtnEl = sidebarEl.parentElement.querySelector('.show-tree-sidebar-button');
  const containerClassList = sidebarEl.parentElement.classList;
  containerClassList.toggle('repo-grid-tree-sidebar', visibility);
  containerClassList.toggle('repo-grid-filelist-only', !visibility);
  toggleElem(sidebarEl, visibility);
  toggleElem(showBtnEl, !visibility);

  if (!sidebarEl.hasAttribute('data-is-signed')) return;

  // save to session
  await PUT('/repo/preferences', {
    data: {
      show_file_view_tree_sidebar: visibility,
    },
  });
}

function childrenLoader(sidebarEl: HTMLElement) {
  return async (path: string, recursive?: boolean) => {
    const fileTree = sidebarEl.querySelector('#view-file-tree');
    const apiBaseUrl = fileTree.getAttribute('data-api-base-url');
    const refTypeNameSubURL = fileTree.getAttribute('data-current-ref-type-name-sub-url');
    const response = await GET(encodeURI(`${apiBaseUrl}/tree/${refTypeNameSubURL}/${path ?? ''}?recursive=${recursive ?? false}`));
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
  const contentEl = sidebarEl.parentElement.querySelector('.repo-home-filelist');
  contentEl.innerHTML = await response.text();
  reloadContentScript(sidebarEl, contentEl);
}

function reloadContentScript(sidebarEl: HTMLElement, contentEl: Element) {
  contentEl.querySelector('.show-tree-sidebar-button')?.addEventListener('click', () => {
    toggleSidebar(sidebarEl, true);
  });
}

export function initViewFileTreeSidebar() {
  registerGlobalInitFunc('initViewFileTreeSidebar', async (el: HTMLElement) => {
    el.querySelector('.hide-tree-sidebar-button').addEventListener('click', () => {
      toggleSidebar(el, false);
    });
    el.parentElement.querySelector('.repo-home-filelist .show-tree-sidebar-button').addEventListener('click', () => {
      toggleSidebar(el, true);
    });

    const fileTree = el.querySelector('#view-file-tree');
    const baseUrl = fileTree.getAttribute('data-api-base-url');
    const treePath = fileTree.getAttribute('data-tree-path');
    const refType = fileTree.getAttribute('data-current-ref-type');
    const refName = fileTree.getAttribute('data-current-ref-short-name');
    const refString = (refType ? (`/${refType}`) : '') + (refName ? (`/${refName}`) : '');

    const selectedItem = ref(getSelectedPath(refString));

    const files = await childrenLoader(el)(treePath, true);

    fileTree.classList.remove('is-loading');
    const fileTreeView = createApp(ViewFileTree, {files, selectedItem, loadChildren: childrenLoader(el), loadContent: (path: string) => {
      window.history.pushState(null, null, encodeURI(`${baseUrl}/src${refString}/${path}`));
      selectedItem.value = path;
      loadContent(el);
    }});
    fileTreeView.mount(fileTree);

    if (!window.popstateListenerForViewFilePageAdded) {
      window.addEventListener('popstate', () => {
        selectedItem.value = getSelectedPath(refString);
        loadContent(el);
      });
      window.popstateListenerForViewFilePageAdded = true;
    }
  });
}

function getSelectedPath(ref: string) {
  const path = decodeURI(new URL(window.location.href).pathname);
  return path.substring(path.indexOf(ref) + ref.length + 1);
}
