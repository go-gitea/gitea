import {reactive} from 'vue';
import {GET} from '../modules/fetch.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {createElementFromHTML} from '../utils/dom.ts';

export function createViewFileTreeStore(props: { repoLink: string, treePath: string, currentRefNameSubURL: string}) {
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
        const svgContainer = createElementFromHTML('<div class="global-svg-icon-pool tw-hidden"></div>');
        svgContainer.innerHTML = poolSvgs.join('');
        document.body.append(svgContainer);
      }
      return json.fileTreeNodes ?? null;
    },

    async loadViewContent(url: string) {
      url = url.includes('?') ? url.replace('?', '?only_content=true') : `${url}?only_content=true`;
      const response = await GET(url);
      document.querySelector('.repo-view-content').innerHTML = await response.text();
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
