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
  permissions?: string[];
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
      const allowedHosts = collectAllowedHosts(meta, fileUrl);
      await withNetworkRestrictions(allowedHosts, async () => {
        const remote = await loadRemoteModule(meta);
        await remote.render(container, fileUrl, options);
      });
    },
  };
}

type RestoreFn = () => void;

function collectAllowedHosts(meta: RemotePluginMeta, fileUrl: string): Set<string> {
  const hosts = new Set<string>();
  const addHost = (value?: string | null) => {
    if (!value) return;
    hosts.add(value.toLowerCase());
  };

  addHost(parseHost(fileUrl));
  for (const perm of meta.permissions ?? []) {
    addHost(normalizeHost(perm));
  }
  return hosts;
}

function normalizeHost(host: string | null | undefined): string | null {
  if (!host) return null;
  return host.trim().toLowerCase();
}

function parseHost(value: string | URL | null | undefined): string | null {
  if (!value) return null;
  try {
    const url = value instanceof URL ? value : new URL(value, window.location.href);
    return normalizeHost(url.host);
  } catch {
    return null;
  }
}

function ensureAllowedHost(kind: string, url: URL, allowedHosts: Set<string>): void {
  const host = normalizeHost(url.host);
  if (!host || allowedHosts.has(host)) {
    return;
  }
  throw new Error(`Render plugin network request for ${kind} blocked: ${host} is not in the declared permissions`);
}

function resolveRequestURL(input: RequestInfo | URL): URL {
  if (typeof Request !== 'undefined' && input instanceof Request) {
    return new URL(input.url, window.location.href);
  }
  if (input instanceof URL) {
    return new URL(input.toString(), window.location.href);
  }
  return new URL(input as string, window.location.href);
}

async function withNetworkRestrictions(allowedHosts: Set<string>, fn: () => Promise<void>): Promise<void> {
  const restoreFns: RestoreFn[] = [];
  const register = (restorer: RestoreFn | null | undefined) => {
    if (restorer) {
      restoreFns.push(restorer);
    }
  };

  register(patchFetch(allowedHosts));
  register(patchXHR(allowedHosts));
  register(patchSendBeacon(allowedHosts));
  register(patchWebSocket(allowedHosts));
  register(patchEventSource(allowedHosts));

  try {
    await fn();
  } finally {
    while (restoreFns.length > 0) {
      const restore = restoreFns.pop();
      restore?.();
    }
  }
}

function patchFetch(allowedHosts: Set<string>): RestoreFn {
  const originalFetch = window.fetch;
  const guarded = (input: RequestInfo | URL, init?: RequestInit) => {
    const target = resolveRequestURL(input);
    ensureAllowedHost('fetch', target, allowedHosts);
    return originalFetch.call(window, input as any, init);
  };
  window.fetch = guarded as typeof window.fetch;
  return () => {
    window.fetch = originalFetch;
  };
}

function patchXHR(allowedHosts: Set<string>): RestoreFn {
  const originalOpen = XMLHttpRequest.prototype.open;
  function guardedOpen(this: XMLHttpRequest, method: string, url: string | URL, async?: boolean, user?: string | null, password?: string | null) {
    const target = url instanceof URL ? url : new URL(url, window.location.href);
    ensureAllowedHost('XMLHttpRequest', target, allowedHosts);
    return originalOpen.call(this, method, url as any, async ?? true, user ?? undefined, password ?? undefined);
  }
  XMLHttpRequest.prototype.open = guardedOpen;
  return () => {
    XMLHttpRequest.prototype.open = originalOpen;
  };
}

function patchSendBeacon(allowedHosts: Set<string>): RestoreFn | null {
  if (typeof navigator.sendBeacon !== 'function') {
    return null;
  }
  const original = navigator.sendBeacon;
  const bound = original.bind(navigator);
  navigator.sendBeacon = ((url: string | URL, data?: BodyInit | null) => {
    const target = url instanceof URL ? url : new URL(url, window.location.href);
    ensureAllowedHost('sendBeacon', target, allowedHosts);
    return bound(url as any, data);
  }) as typeof navigator.sendBeacon;
  return () => {
    navigator.sendBeacon = original;
  };
}

function patchWebSocket(allowedHosts: Set<string>): RestoreFn {
  const OriginalWebSocket = window.WebSocket;
  const GuardedWebSocket = function(url: string | URL, protocols?: string | string[]) {
    const target = url instanceof URL ? url : new URL(url, window.location.href);
    ensureAllowedHost('WebSocket', target, allowedHosts);
    return new OriginalWebSocket(url as any, protocols);
  } as unknown as typeof WebSocket;
  GuardedWebSocket.prototype = OriginalWebSocket.prototype;
  Object.setPrototypeOf(GuardedWebSocket, OriginalWebSocket);
  window.WebSocket = GuardedWebSocket;
  return () => {
    window.WebSocket = OriginalWebSocket;
  };
}

function patchEventSource(allowedHosts: Set<string>): RestoreFn | null {
  if (typeof window.EventSource !== 'function') {
    return null;
  }
  const OriginalEventSource = window.EventSource;
  const GuardedEventSource = function(url: string | URL, eventSourceInitDict?: EventSourceInit) {
    const target = url instanceof URL ? url : new URL(url, window.location.href);
    ensureAllowedHost('EventSource', target, allowedHosts);
    return new OriginalEventSource(url as any, eventSourceInitDict);
  } as unknown as typeof EventSource;
  GuardedEventSource.prototype = OriginalEventSource.prototype;
  Object.setPrototypeOf(GuardedEventSource, OriginalEventSource);
  window.EventSource = GuardedEventSource;
  return () => {
    window.EventSource = OriginalEventSource;
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
