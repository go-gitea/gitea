import {sleep} from '../utils.ts';

const {appSubUrl} = window.config;

export async function logoutFromWorker(): Promise<void> {
  // wait for a while because other requests (eg: logout) may be in the flight
  await sleep(5000);
  window.location.href = `${appSubUrl}/`;
}
