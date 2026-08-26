export type TimeoutId = ReturnType<typeof setTimeout>;
export type IntervalId = ReturnType<typeof setInterval>;

export type Intent = 'error' | 'warning' | 'info' | 'success';

export type Mention = {
  key: string,
  value: string,
  name: string,
  fullname: string,
  avatar: string,
};

export type RequestData = FormData | URLSearchParams | Record<string, unknown> | unknown[];

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

export type Label = {
  id: number,
  name: string,
  exclusive: boolean,
  is_archived: boolean,
  color: string,
  description: string,
  url: string,
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
    id: number,
    name: string,
    owner: string,
    full_name: string,
  },
  labels: Array<Label>,
};

// derived from the "$" global because "no-undef" can't see the ambient "JQuery" type
export type JQueryElem<T extends HTMLElement = HTMLElement> = ReturnType<typeof $<T>>;

export type FomanticApiResponse<T> = {success: boolean, results: T[]};

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
export type ServerUserEventMessage =
  {eventType: 'notification-count', eventData: {count: number}} |
  {eventType: 'stopwatches', eventData: Array<StopwatchData>} |
  {eventType: 'logout'};

export const serverUserEventTypes = ['notification-count', 'stopwatches', 'logout'] as const satisfies ReadonlyArray<ServerUserEventMessage['eventType']>;

export type UserEventMessage = ServerUserEventMessage |
  {eventType: 'worker-unavailable'} |
  {eventType: 'worker-connected'};

export type UserEventType = UserEventMessage['eventType'];

export type WorkerEventMessage =
  {workerEvent: 'error', message: string} |
  {workerEvent: 'close'};

export type WorkerInboundMessage =
  {msgType: 'user-event', msgData: UserEventMessage} |
  {msgType: 'worker-event', msgData: WorkerEventMessage};

export type SharedWorkerControlMessage = {
  type: 'start',
  url: string,
  showDebugLog: boolean,
} | {type: 'close'};
