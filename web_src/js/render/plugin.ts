export type FileRenderPlugin = {
  // unique plugin name
  name: string;

  // test if plugin can handle a specified file
  canHandle: (filename: string, mimeType: string) => boolean;

  // render file content
  render: (container: HTMLElement, fileUrl: string, options?: any) => Promise<void>;
}
