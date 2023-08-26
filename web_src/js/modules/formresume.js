import {persistResumableFields, restoreResumableFields, setForm} from '@github/session-resume';

// This will persist any unsubmitted form fields into sessionStorage which have a
// js-session-resumable class on them
export function initGlobalFormResume() {
  window.addEventListener('submit', setForm, {capture: true});

  window.addEventListener('pageshow', () => {
    restoreResumableFields(window.location.pathname);
  });

  window.addEventListener('pagehide', () => {
    persistResumableFields(window.location.pathname);
  });
}
