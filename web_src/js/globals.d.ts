interface JQuery {
  areYouSure: any, // jquery.are-you-sure
  fomanticExt: any; // fomantic extension
  api: any, // fomantic
  dimmer: any, // fomantic
  dropdown: any; // fomantic
  modal: any; // fomantic
  tab: any; // fomantic
  transition: any, // fomantic
  search: any, // fomantic
}

interface JQueryStatic {
  api: any, // fomantic
}

interface Element {
  _tippy: import('tippy.js').Instance;
}

interface Window {
  config: {
    appUrl: string,
    appSubUrl: string,
    assetVersionEncoded: string,
    assetUrlPrefix: string,
    runModeIsProd: boolean,
    customEmojis: Record<string, string>,
    pageData: Record<string, any>,
    notificationSettings: Record<string, any>,
    enableTimeTracking: boolean,
    mentionValues: Array<import('./types.ts').MentionValue>,
    mermaidMaxSourceCharacters: number,
    i18n: Record<string, string>,
  },
  $: typeof import('@types/jquery'),
  jQuery: typeof import('@types/jquery'),
  htmx: typeof import('htmx.org').default,
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
  codeEditors: any[], // export editor for customization
  localUserSettings: typeof import('./modules/user-settings.ts').localUserSettings,

  // various captcha plugins
  grecaptcha: any,
  turnstile: any,
  hcaptcha: any,

  // do not add more properties here unless it is a must
}
