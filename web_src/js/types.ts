export type IntervalId = ReturnType<typeof setInterval>;

export type Intent = 'error' | 'warning' | 'info';

export type Mention = {
  key: string,
  value: string,
  name: string,
  fullname: string,
  avatar: string,
};

export type RequestData = string | FormData | URLSearchParams | Record<string, any>;

export type RequestOpts = {
  data?: RequestData,
} & RequestInit;

export type RepoOwnerPathInfo = {
  ownerName: string,
  repoName: string,
};

export type IssuePathInfo = {
  ownerName: string,
  repoName: string,
  pathType: string,
  indexString: string,
};

export type IssuePageInfo = {
  repoLink: string,
  repoId: number,
  issueNumber: number,
  issueDependencySearchType: string,
};

export type Issue = {
  id: number,
  number: number,
  title: string,
  body: string,
  state: 'open' | 'closed',
  created_at: string,
  html_url: string,
  pull_request?: {
    draft: boolean;
    merged: boolean;
  },
  repository: {
    full_name: string,
    html_url: string,
  },
  labels: Array<string>,
};

export type FomanticInitFunction = {
  settings?: Record<string, any>,
  (...args: any[]): any,
};

export type GitRefType = 'branch' | 'tag';

export type Promisable<T> = T | Promise<T>; // stricter than type-fest which uses PromiseLike

export type StopwatchData = {
  repo_owner_name: string,
  repo_name: string,
  issue_index: number,
  seconds: number,
};

// keep in sync with services/websocket/events.go
export type ServerEventMessage =
  {type: 'notification-count', count: number} |
  {type: 'stopwatches', data: Array<StopwatchData>} |
  {type: 'logout', data: 'here' | 'elsewhere'};

// `satisfies` makes adding a new variant to ServerEventMessage without updating this array a type error.
export const serverEventTypes = ['notification-count', 'stopwatches', 'logout'] as const satisfies ReadonlyArray<ServerEventMessage['type']>;

export type UserEventMessage =
  ServerEventMessage |
  {type: 'push-unavailable'} |
  {type: 'ws-connected'};

export type UserEventType = UserEventMessage['type'];

export type WorkerInboundMessage =
  UserEventMessage |
  {type: 'error', message?: string} |
  {type: 'close'};
