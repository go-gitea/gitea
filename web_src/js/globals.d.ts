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
  htmx: typeof import('htmx.org').default,
  _globalHandlerErrors: Array<ErrorEvent & PromiseRejectionEvent> & {
    _inited: boolean,
    push: (e: ErrorEvent & PromiseRejectionEvent) => void | number,
  },
  codeEditors: any[], // export editor for customization
  localUserSettings: typeof import('./modules/user-settings.ts').localUserSettings,

  MonacoEnvironment?: {
    getWorker: (workerId: string, label: string) => Worker,
  },

  // various captcha plugins
  grecaptcha: any,
  turnstile: any,
  hcaptcha: any,

  // do not add more properties here unless it is a must
}

declare module '*?worker' {
  const workerConstructor: new () => Worker;
  export default workerConstructor;
}
