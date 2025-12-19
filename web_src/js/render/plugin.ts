export type FileRenderOptions = {
  /** MIME type reported by the backend (may be empty). */
  mimeType?: string;
  /** First bytes of the file as raw bytes (<= 1 KiB). */
  headChunk?: Uint8Array | null;
  /** Additional plugin-specific options. */
  [key: string]: any;
};

export type FileRenderPlugin = {
  // unique plugin name
  name: string;

  // test if plugin can handle a specified file
  canHandle: (filename: string, mimeType: string, headChunk?: Uint8Array | null) => boolean;

  // render file content
  render: (container: HTMLElement, fileUrl: string, options?: FileRenderOptions) => Promise<void>;
};
