// Wire contract with services/websocket/events.go — keep in sync.
export type UserEventType = 'notification-count' | 'stopwatches' | 'logout' | 'ws-opened' | 'push-unavailable';

export type UserEventMessage = {
  type: UserEventType,
  data: string,
};

export type WorkerInboundMessage = {
  type: UserEventType | 'error' | 'close' | 'status',
  data?: any,
  message?: string,
};
