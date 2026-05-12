// there are 2 kinds of plugins:
// * "inplace" plugins: render file content in-place, e.g. PDF viewer
// * "frontend" plugins: render file content in a separate iframe by a huge frontend library (need to protect from XSS risks)
// TODO: render plugin enhancements, not needed at the moment, leave the problems to the future when the problems actually come:
//  1. provide the prefetched file head bytes to let the plugin decide whether to render or not
//  2. multiple plugins can render the same file, so we should not assume only one plugin will render it

export type InplaceRenderPlugin = {
  name: string;
  canHandle: (filename: string, mimeType: string) => boolean;
  render: (container: HTMLElement, fileUrl: string, options?: any) => Promise<void>;
};

export type FrontendRenderOptions = {
  container: HTMLElement;
  treePath: string;
  contentString(): string;
  contentBytes(): Uint8Array<ArrayBuffer>;
};

export type FrontendRenderFunc = (opts: FrontendRenderOptions) => Promise<boolean>;
