// Wire-format event type names — must match services/websocket/events.go on
// the server. Also used for the SharedWorker<->page messages so that page code
// never hardcodes these strings.
export const USER_EVENT_NOTIFICATION_COUNT = 'notification-count';
export const USER_EVENT_STOPWATCHES = 'stopwatches';
export const USER_EVENT_LOGOUT = 'logout';
export const USER_EVENT_WS_OPENED = 'ws-opened';
export const USER_EVENT_PUSH_UNAVAILABLE = 'push-unavailable';
