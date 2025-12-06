import type {FileRenderPlugin} from '../plugin.ts';
import {globCompile} from '../../utils/glob.ts';

type RemotePluginMeta = {
  schemaVersion: number;
  id: string;
  name: string;
  version: string;
  description: string;
  entryUrl: string;
  assetsBaseUrl: string;
  filePatterns: string[];
};

type RemotePluginModule = {
  render: (container: HTMLElement, fileUrl: string, options?: any) => void | Promise<void>;
};

const moduleCache = new Map<string, Promise<RemotePluginModule>>();
const SUPPORTED_SCHEMA_VERSION = 1;

async function fetchRemoteMetadata(): Promise<RemotePluginMeta[]> {
  const base = window.config.appSubUrl || '';
  const response = await window.fetch(`${base}/assets/render-plugins/index.json`, {headers: {'Accept': 'application/json'}});
  if (!response.ok) {
    throw new Error(`Failed to load render plugin metadata (${response.status})`);
  }
  return response.json() as Promise<RemotePluginMeta[]>;
}

async function loadRemoteModule(meta: RemotePluginMeta): Promise<RemotePluginModule> {
  let cached = moduleCache.get(meta.id);
  if (!cached) {
    cached = (async () => {
      try {
        const mod = await import(/* webpackIgnore: true */ meta.entryUrl);
        const exported = (mod?.default ?? mod) as RemotePluginModule | undefined;
        if (!exported || typeof exported.render !== 'function') {
          throw new Error(`Plugin ${meta.id} does not export a render() function`);
        }
        return exported;
      } catch (err) {
        moduleCache.delete(meta.id);
        throw err;
      }
    })();
    moduleCache.set(meta.id, cached);
  }
  return cached;
}

function createMatcher(patterns: string[]) {
  const compiled = patterns.map((pattern) => {
    const normalized = pattern.toLowerCase();
    try {
      return globCompile(normalized);
    } catch (err) {
      console.error('Failed to compile render plugin glob pattern', pattern, err);
      return null;
    }
  }).filter(Boolean) as ReturnType<typeof globCompile>[];
  return (filename: string) => {
    const lower = filename.toLowerCase();
    return compiled.some((glob) => glob.regexp.test(lower));
  };
}

function wrapRemotePlugin(meta: RemotePluginMeta): FileRenderPlugin {
  const matcher = createMatcher(meta.filePatterns);
  return {
    name: meta.name,
    canHandle(filename: string, _mimeType: string, _headChunk?: Uint8Array | null) {
      return matcher(filename);
    },
    async render(container, fileUrl, options) {
      const remote = await loadRemoteModule(meta);
      await remote.render(container, fileUrl, options);
    },
  };
}

export async function loadDynamicRenderPlugins(): Promise<FileRenderPlugin[]> {
  try {
    const metadata = await fetchRemoteMetadata();
    return metadata.filter((meta) => {
      if (meta.schemaVersion !== SUPPORTED_SCHEMA_VERSION) {
        console.warn(`Render plugin ${meta.id} ignored due to incompatible schemaVersion ${meta.schemaVersion}`);
        return false;
      }
      return true;
    }).map((meta) => wrapRemotePlugin(meta));
  } catch (err) {
    console.error('Failed to load dynamic render plugins', err);
    return [];
  }
}
