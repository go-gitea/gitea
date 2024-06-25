import {sleep} from '../utils.js';

const {appSubUrl} = window.config;

export async function logoutFromWorker() {
  // wait for a while because other requests (eg: logout) may be in the flight
  await sleep(5000);
  window.location.href = `${appSubUrl}/`;
}
