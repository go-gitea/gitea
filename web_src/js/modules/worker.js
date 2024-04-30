const {appSubUrl} = window.config;

export function logoutFromWorker() {
  // wait for a while because other requests (eg: logout) may be in the flight
  setTimeout(() => {
    window.location.href = `${appSubUrl}/`;
  }, 5000);
}
