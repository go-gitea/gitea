// Stub APIs not implemented by happy-dom but needed by dependencies
// XPathEvaluator is used by htmx at module evaluation time
// TODO: Remove after https://github.com/capricorn86/happy-dom/pull/2103 is released
if (!globalThis.XPathEvaluator) {
  globalThis.XPathEvaluator = class {
    createExpression() { return {evaluate: () => ({iterateNext: () => null})} }
  } as any;
}

// Dynamic import so polyfills above are applied before htmx evaluates
await import('./globals.ts');

window.config = {
  appUrl: 'http://localhost:3000/',
  appSubUrl: '',
  assetUrlPrefix: '',
  sharedWorkerUri: '',
  runModeIsProd: true,
  customEmojis: {},
  pageData: {},
  notificationSettings: {MinTimeout: 0, TimeoutStep: 0, MaxTimeout: 0, EventSourceUpdateTime: 0},
  enableTimeTracking: true,
  mermaidMaxSourceCharacters: 5000,
  i18n: {},
};

export {}; // mark as module for top-level await
