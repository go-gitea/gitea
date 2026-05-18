import type {FrontendRenderFunc, FrontendRenderOptions} from './render/plugin.ts';

type LazyLoadFunc = () => Promise<{frontendRender: FrontendRenderFunc}>;

// It must use a wrapper function to avoid the "import" statement being treated
// as static import and cause the all plugins being loaded together,
// We only need to load the plugins we need.
const frontendPlugins: Record<string, LazyLoadFunc> = {
  'viewer-3d': () => import('./render/plugins/frontend-viewer-3d.ts'),
  'openapi-swagger': () => import('./render/plugins/frontend-openapi-swagger.ts'),
};

class Options implements FrontendRenderOptions {
  container: HTMLElement;
  treePath: string;
  rawEncoding: string;
  rawString: string;
  cachedBytes: Uint8Array<ArrayBuffer> | null = null;
  cachedString: string | null = null;
  constructor(container: HTMLElement, treePath: string, rawEncoding: string, rawString: string) {
    this.container = container;
    this.treePath = treePath;
    this.rawEncoding = rawEncoding;
    this.rawString = rawString;
  }
  decodeBase64(): Uint8Array<ArrayBuffer> {
    return Uint8Array.from(atob(this.rawString), (c) => c.charCodeAt(0));
  }
  contentBytes(): Uint8Array<ArrayBuffer> {
    if (this.cachedBytes === null) {
      this.cachedBytes = this.rawEncoding === 'base64' ? this.decodeBase64() : new TextEncoder().encode(this.rawString);
    }
    return this.cachedBytes;
  }
  contentString(): string {
    if (this.cachedString === null) {
      this.cachedString = this.rawEncoding === 'base64' ? new TextDecoder('utf-8').decode(this.decodeBase64()) : this.rawString;
    }
    return this.cachedString;
  }
}

async function initFrontendExternalRender() {
  const viewerContainer = document.querySelector<HTMLElement>('#frontend-render-viewer')!;
  const renderNames = viewerContainer.getAttribute('data-frontend-renders')!.split(' ');
  const fileTreePath = viewerContainer.getAttribute('data-file-tree-path')!;

  const fileDataElem = document.querySelector<HTMLTextAreaElement>('#frontend-render-data')!;
  fileDataElem.remove();
  const fileDataContent = fileDataElem.value;
  const fileDataEncoding = fileDataElem.getAttribute('data-content-encoding')!;
  const opts = new Options(viewerContainer, fileTreePath, fileDataEncoding, fileDataContent);

  let found = false;
  for (const name of renderNames) {
    if (!(name in frontendPlugins)) continue;
    const plugin = await frontendPlugins[name]();
    found = true;
    if (await plugin.frontendRender(opts)) break;
  }

  if (!found) {
    viewerContainer.textContent = 'No frontend render plugin found for this file, but backend declares that there must be one, there must be a bug';
  }
}

initFrontendExternalRender();
