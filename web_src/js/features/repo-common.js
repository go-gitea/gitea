import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

const {csrfToken} = window.config;

function getArchive($target, url, first) {
  $.ajax({
    url,
    type: 'POST',
    data: {
      _csrf: csrfToken,
    },
    complete(xhr) {
      if (xhr.status === 200) {
        if (!xhr.responseJSON) {
          // XXX Shouldn't happen?
          $target.closest('.dropdown').children('i').removeClass('loading');
          return;
        }

        if (!xhr.responseJSON.complete) {
          $target.closest('.dropdown').children('i').addClass('loading');
          // Wait for only three quarters of a second initially, in case it's
          // quickly archived.
          setTimeout(() => {
            getArchive($target, url, false);
          }, first ? 750 : 2000);
        } else {
          // We don't need to continue checking.
          $target.closest('.dropdown').children('i').removeClass('loading');
          window.location.href = url;
        }
      }
    },
  });
}

export function initRepoArchiveLinks() {
  $('.archive-link').on('click', function (event) {
    event.preventDefault();
    const url = $(this).attr('href');
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
  $dropdown.dropdown({
    fullTextSearch: 'exact',
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice.attr('data-url')) {
        window.location.href = $choice.attr('data-url');
      }
    },
    message: {noResults: $dropdown.attr('data-no-results')},
  });
}
