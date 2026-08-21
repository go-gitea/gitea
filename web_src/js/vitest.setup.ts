import './globals.ts';

window.config = {
  appUrl: `${window.location.origin}/`,
  appSubUrl: '',
  assetUrlPrefix: '/assets',
  sharedWorkerUri: '',
  runModeIsProd: true,
  customEmojis: {},
  pageData: {},
  notificationSettings: {MinTimeout: 0, TimeoutStep: 0, MaxTimeout: 0},
  enableTimeTracking: true,
  mermaidMaxSourceCharacters: 5000,
  i18n: {},
  frontendInited: false,
};
