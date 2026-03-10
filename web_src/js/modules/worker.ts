const {appSubUrl} = window.config;

export function logoutFromWorker(): void {
  window.location.href = `${appSubUrl}/`;
}
