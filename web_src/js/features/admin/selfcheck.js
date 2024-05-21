import {toggleElem} from '../../utils/dom.js';
import {POST} from '../../modules/fetch.js';

const {appSubUrl} = window.config;

export async function initAdminSelfCheck() {
  const elCheckByFrontend = document.querySelector('#self-check-by-frontend');
  if (!elCheckByFrontend) return;

  const elContent = document.querySelector('.page-content.admin .admin-setting-content');

  // send frontend self-check request
  const resp = await POST(`${appSubUrl}/admin/self_check`, {
    data: new URLSearchParams({
      location_origin: window.location.origin,
      now: Date.now(), // TODO: check time difference between server and client
    }),
  });
  const json = await resp.json();
  toggleElem(elCheckByFrontend, Boolean(json.problems?.length));
  for (const problem of json.problems ?? []) {
    const elProblem = document.createElement('div');
    elProblem.classList.add('ui', 'warning', 'message');
    elProblem.textContent = problem;
    elCheckByFrontend.append(elProblem);
  }

  // only show the "no problem" if there is no visible "self-check-problem"
  const hasProblem = Boolean(elContent.querySelectorAll('.self-check-problem:not(.tw-hidden)').length);
  toggleElem(elContent.querySelector('.self-check-no-problem'), !hasProblem);
}
