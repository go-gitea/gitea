/**
 * File Render Plugin System
 *
 * This module provides a plugin architecture for rendering different file types
 * in the browser without requiring backend support for identifying file types.
 */

/**
 * Interface for file render plugins
 */
export type FileRenderPlugin = {
  // unique plugin name
  name: string;

  // test if plugin can handle specified file
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
export async function applyRenderPlugin(container: HTMLElement): Promise<boolean> {
  try {
    // get file info from container element
    const filename = container.getAttribute('data-filename') || '';
    const fileUrl = container.getAttribute('data-url') || '';

    if (!filename || !fileUrl) {
      console.warn('Missing filename or file URL for renderer');
      return false;
    }

    // get mime type (optional)
    const mimeType = container.getAttribute('data-mime-type') || '';

    // find plugin that can handle this file
    const plugin = findPlugin(filename, mimeType);
    if (!plugin) {
      return false;
    }

    // apply plugin to render file
    await plugin.render(container, fileUrl);
    return true;
  } catch (error) {
    console.error('Error applying render plugin:', error);
    return false;
  }
}
