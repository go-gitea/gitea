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
    assetUrlPrefix: string,
    sharedWorkerUri: string,
    runModeIsProd: boolean,
    customEmojis: Record<string, string>,
    pageData: Record<string, any> & {
      adminUserListSearchForm?: {
        SortType: string,
        StatusFilterMap: Record<string, string>,
      },
      citationFileContent?: string,
      prReview?: {
        numberOfFiles: number,
        numberOfViewedFiles: number,
      },
      DiffFileTree?: import('./modules/diff-file.ts').DiffFileTreeData,
      FolderIcon?: string,
      FolderOpenIcon?: string,
      repoLink?: string,
      repoActivityTopAuthors?: any[],
      pullRequestMergeForm?: Record<string, any>,
      dashboardRepoList?: Record<string, any>,
    },
    notificationSettings: {
      MinTimeout: number,
      TimeoutStep: number,
      MaxTimeout: number,
      EventSourceUpdateTime: number,
    },
    enableTimeTracking: boolean,
    mermaidMaxSourceCharacters: number,
    i18n: Record<string, string>,
  },
  $: JQueryStatic,
  jQuery: JQueryStatic,
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
  localUserSettings: typeof import('./modules/user-settings.ts').localUserSettings,

  // various captcha plugins
  grecaptcha: any,
  turnstile: any,
  hcaptcha: any,

  // Make IIFE private functions can be tested in unit tests, without exposing the IIFE module to global scope.
  // Otherwise, when using "export" in IIFE code, the compiled JS will inject global "var externalRenderHelper = ..."
  // which is not expected and may cause conflicts with other modules.
  testModules: {
    externalRenderHelper?: {
      isValidCssColor(s: string | null): boolean,
    }
  }

  // do not add more properties here unless it is a must
}

declare module '*?worker' {
  const workerConstructor: new () => Worker;
  export default workerConstructor;
}
