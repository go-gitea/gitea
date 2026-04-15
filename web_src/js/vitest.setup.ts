import './globals.ts';

window.config = {
  appUrl: 'http://localhost:3000/',
  appSubUrl: '',
  assetUrlPrefix: '/assets',
  sharedWorkerUri: '',
  runModeIsProd: true,
  customEmojis: {},
  pageData: {},
  notificationSettings: {MinTimeout: 0, TimeoutStep: 0, MaxTimeout: 0, EventSourceUpdateTime: 0},
  enableTimeTracking: true,
  mermaidMaxSourceCharacters: 5000,
  i18n: {},
};

window.testModules = {};

export {}; // mark as module for top-level await
