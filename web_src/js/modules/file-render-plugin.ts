/**
 * File Render Plugin System
 *
 * This module provides a plugin architecture for rendering different file types
 * in the browser without requiring backend support for identifying file types.
 */
export type FileRenderPlugin = {
  // unique plugin name
  name: string;

  // test if plugin can handle a specified file
  canHandle: (filename: string, mimeType: string) => boolean;

  // render file content
  render: (container: HTMLElement, fileUrl: string, options?: any) => Promise<void>;
}

const plugins: FileRenderPlugin[] = [];

export function registerFileRenderPlugin(plugin: FileRenderPlugin): void {
  plugins.push(plugin);
}

export function findFileRenderPlugin(filename: string, mimeType: string): FileRenderPlugin | null {
  return plugins.find((plugin) => plugin.canHandle(filename, mimeType)) || null;
}
