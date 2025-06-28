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

// store registered render plugins
const plugins: FileRenderPlugin[] = [];

/**
 * register a file render plugin
 */
export function registerFileRenderPlugin(plugin: FileRenderPlugin): void {
  plugins.push(plugin);
}

/**
 * find suitable render plugin by filename and mime type
 */
function findPlugin(filename: string, mimeType: string): FileRenderPlugin | null {
  return plugins.find((plugin) => plugin.canHandle(filename, mimeType)) || null;
}

/**
 * apply render plugin to specified container
 */
export async function applyRenderPlugin(container: HTMLElement, rawFileLink: string): Promise<boolean> {
  try {
    const mimeType = container.getAttribute('data-mime-type') || '';
    const plugin = findPlugin(rawFileLink, mimeType);
    if (!plugin) return false;

    container.classList.add('is-loading');
    await plugin.render(container, rawFileLink);
    return true;
  } finally {
    container.classList.remove('is-loading');
  }
}
