import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';
import {POST} from '../modules/fetch.js';

async function getArchive($target, url, first) {
  const dropdownBtn = $target[0].closest('.ui.dropdown.button') ?? $target[0].closest('.ui.dropdown.btn');

  try {
    dropdownBtn.classList.add('is-loading');
    const response = await POST(url);
    if (response.status === 200) {
      const data = await response.json();
      if (!data) {
        // XXX Shouldn't happen?
        dropdownBtn.classList.remove('is-loading');
        return;
      }

      if (!data.complete) {
        // Wait for only three quarters of a second initially, in case it's
        // quickly archived.
        setTimeout(() => {
          getArchive($target, url, false);
        }, first ? 750 : 2000);
      } else {
        // We don't need to continue checking.
        dropdownBtn.classList.remove('is-loading');
        window.location.href = url;
      }
    }
  } catch {
    dropdownBtn.classList.remove('is-loading');
  }
}

export function initRepoArchiveLinks() {
  $('.archive-link').on('click', function (event) {
    event.preventDefault();
    const url = this.getAttribute('href');
    if (!url) return;
    getArchive($(event.target), url, true);
  });
}

export function initRepoCloneLink() {
  const $repoCloneSsh = $('#repo-clone-ssh');
  const $repoCloneHttps = $('#repo-clone-https');
  const $inputLink = $('#repo-clone-url');

  if ((!$repoCloneSsh.length && !$repoCloneHttps.length) || !$inputLink.length) {
    return;
  }

  $repoCloneSsh.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    window.updateCloneStates();
  });
  $repoCloneHttps.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
    window.updateCloneStates();
  });

  $inputLink.on('focus', () => {
    $inputLink.trigger('select');
  });
}

export function initRepoCommonBranchOrTagDropdown(selector) {
  $(selector).each(function () {
    const $dropdown = $(this);
    $dropdown.find('.reference.column').on('click', function () {
      hideElem($dropdown.find('.scrolling.reference-list-menu'));
      showElem($($(this).data('target')));
      return false;
    });
  });
}

export function initRepoCommonFilterSearchDropdown(selector) {
  const $dropdown = $(selector);
  if (!$dropdown.length) return;

  $dropdown.dropdown({
    fullTextSearch: 'exact',
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice[0].getAttribute('data-url')) {
        window.location.href = $choice[0].getAttribute('data-url');
      }
    },
    message: {noResults: $dropdown[0].getAttribute('data-no-results')},
  });
}
