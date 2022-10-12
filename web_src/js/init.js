// This script is for critical JS that needs to run as soon as possible.
// Do not import any dependencies and keep to vanilla JS only.

// This function runs before DOMContentLoaded and checks if most of the page
// has loaded so we can do DOM mutations before anything is painted on the screen.
requestAnimationFrame(function init(elapsed) {
  if (elapsed > 10000) return;
  if (!document.querySelector('script[src*="index.js"]')) return requestAnimationFrame(init);

  // Synchronously set clone button states and urls here to avoid flickering
  // on page load. initRepoCloneLink calls this when proto changes.
  // this applies the protocol-dependant clone url to all elements with the
  // `js-clone-url` and `js-clone-url-vsc` classes.
  // TODO: This localStorage setting should be moved to backend user config.
  (window.updateCloneStates = function() {
    const httpsBtn = document.getElementById('repo-clone-https');
    const sshBtn = document.getElementById('repo-clone-ssh');
    const value = localStorage.getItem('repo-clone-protocol') || 'https';
    const isSSH = value === 'ssh' && sshBtn || value !== 'ssh' && !httpsBtn;

    if (httpsBtn) httpsBtn.classList[!isSSH ? 'add' : 'remove']('primary');
    if (sshBtn) sshBtn.classList[isSSH ? 'add' : 'remove']('primary');

    const btn = isSSH ? sshBtn : httpsBtn;
    if (!btn) return;

    const link = btn.getAttribute('data-link');
    for (const el of document.getElementsByClassName('js-clone-url')) {
      el[el.nodeName === 'INPUT' ? 'value' : 'textContent'] = link;
    }
    for (const el of document.getElementsByClassName('js-clone-url-vsc')) {
      el.href = `vscode://vscode.git/clone?url=${encodeURIComponent(link)}`;
    }
  })();
});
