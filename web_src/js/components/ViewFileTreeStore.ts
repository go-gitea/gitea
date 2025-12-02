import {reactive} from 'vue';
import {GET} from '../modules/fetch.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {html} from '../utils/html.ts';

export function createViewFileTreeStore(props: {repoLink: string, treePath: string, currentRefNameSubURL: string}) {
  const store = reactive({
    rootFiles: [],
    selectedItem: props.treePath,

    async loadChildren(treePath: string, subPath: string = '') {
      const response = await GET(`${props.repoLink}/tree-view/${props.currentRefNameSubURL}/${pathEscapeSegments(treePath)}?sub_path=${encodeURIComponent(subPath)}`);
      const json = await response.json();
      const poolSvgs = [];
      for (const [svgId, svgContent] of Object.entries(json.renderedIconPool ?? {})) {
        if (!document.querySelector(`.global-svg-icon-pool #${svgId}`)) poolSvgs.push(svgContent);
      }
      if (poolSvgs.length) {
        const svgContainer = createElementFromHTML(html`<div class="global-svg-icon-pool tw-hidden"></div>`);
        svgContainer.innerHTML = poolSvgs.join('');
        document.body.append(svgContainer);
      }
      return json.fileTreeNodes ?? null;
    },

    async loadViewContent(url: string) {
      const u = new URL(url, window.origin);
      u.searchParams.set('only_content', 'true');
      const response = await GET(u.href);
      const elViewContent = document.querySelector('.repo-view-content');
      elViewContent.innerHTML = await response.text();
      const elViewContentData = elViewContent.querySelector('.repo-view-content-data');
      if (!elViewContentData) return; // if error occurs, there is no such element
      const t1 = elViewContentData.getAttribute('data-document-title');
      const t2 = elViewContentData.getAttribute('data-document-title-common');
      document.title = `${t1} - ${t2}`; // follow the format in head.tmpl: <head><title>...</title></head>
    },

    async navigateTreeView(treePath: string) {
      const url = store.buildTreePathWebUrl(treePath);
      window.history.pushState({treePath, url}, null, url);
      store.selectedItem = treePath;
      await store.loadViewContent(url);
    },

    buildTreePathWebUrl(treePath: string) {
      return `${props.repoLink}/src/${props.currentRefNameSubURL}/${pathEscapeSegments(treePath)}`;
    },
  });
  return store;
}
