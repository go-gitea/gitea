export type MentionValue = {
  key: string,
  value: string,
  name: string,
  fullname: string,
  avatar: string,
}

export type Config = {
  appUrl: string,
  appSubUrl: string,
  assetVersionEncoded: string,
  assetUrlPrefix: string,
  runModeIsProd: boolean,
  customEmojis: Record<string, string>,
  csrfToken: string,
  pageData: Record<string, any>,
  notificationSettings: Record<string, any>,
  enableTimeTracking: boolean,
  mentionValues?: MentionValue[],
  mermaidMaxSourceCharacters: number,
  i18n: Record<string, string>,
}

export type Intent = 'error' | 'warning' | 'info';

export type RequestData = string | FormData | URLSearchParams;

export type RequestOpts = {
  data?: RequestData,
} & RequestInit;

export type IssuePathInfo = {
  ownerName: string,
  repoName: string,
  pathType: string,
  indexString?: string,
}

export type IssuePageInfo = {
  repoLink: string,
  repoId: number,
  issueNumber: number,
  issueDependencySearchType: string,
}

export type Issue = {
  id: number;
  number: number;
  title: string;
  state: 'open' | 'closed';
  pull_request?: {
    draft: boolean;
    merged: boolean;
  };
};
