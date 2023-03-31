import $ from 'jquery';
import {hideElem, showElem, toggleElem} from '../utils/dom.js';
import {updateMenuItem, ariaPatchKey} from '../modules/aria/dropdown.js';
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

  // restore animation after first init
  setTimeout(() => {
    $repoCloneSsh.removeClass('gt-no-transition');
    $repoCloneHttps.removeClass('gt-no-transition');
  }, 100);

  $repoCloneSsh.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    window.updateCloneStates();
  });
  $repoCloneHttps.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
    window.updateCloneStates();
  });

  $inputLink.on('focus', () => {
    $inputLink.select();
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

export function initRepoCommonLanguageStats() {
  // Language stats
  if ($('.language-stats').length > 0) {
    $('.language-stats').on('click', (e) => {
      e.preventDefault();
      toggleElem($('.language-stats-details, .repository-menu'));
    });
  }
}

// generate dropdown options for authors search dropdown using fetched data
export function initPostersDropdown() {
  const $authorSearchDropdown = $('.author-search');
  $('#author-search-input').on('input', function(e) {
    e.stopImmediatePropagation();
    fetchPostersData($authorSearchDropdown, false);
  });
  // show all results when clicking on the dropdown
  $authorSearchDropdown.on('click', function(e) {
    fetchPostersData($authorSearchDropdown, true);
  });
}

// isShowAll decides if fetching all data or fetching data with search query
async function fetchPostersData($authorSearchDropdown, isShowAll) {
  if (!$authorSearchDropdown.length) {
    return;
  }
  const baseUrl = $authorSearchDropdown.attr('data-url');
  const url = isShowAll ? baseUrl: `${baseUrl}?q=${$('#author-search-input').val()}`;
  const res = await fetch(url);
  const postersJson = await res.json();
  if (!postersJson) {
    $authorSearchDropdown.addClass('disabled');
    return;
  }
  // get data needed from data- attributes for generating the poster options
  const posterID = $authorSearchDropdown.attr('data-poster-id');
  const isShowFullName = $authorSearchDropdown.attr('data-show-fullname');
  const posterGeneralUrl = $authorSearchDropdown.attr('data-general-poster-url');
  const values = $authorSearchDropdown.dropdown('setting values');
  const $defaultMenu = $(values[0]).find('.menu');
  // remove former options, then append newly searched posters
  $defaultMenu.find(".item:gt(0)").remove();
  for (let i = 0; i < postersJson.length; i++) {
    const {id, avatar_url, username, full_name} = postersJson[i];
    $defaultMenu.append(`<a class="item gt-df${posterID === id ? ' active selected' : ''}" href="${posterGeneralUrl}${id}">
      <img class="ui avatar gt-vm" src="${avatar_url}" title="${username}" width="28" height="28">
      <span class="gt-ellipsis">${username}${isShowFullName === 'true' ? `<span class="search-fullname"> ${full_name}</span>` : ''}</span>
    </a>`);
  }
  // append aria related attributes to newly added menu items
  const $items = $defaultMenu.find('> .item');
  $items.each((_, item) => updateMenuItem($authorSearchDropdown[0], item));
  $authorSearchDropdown[0][ariaPatchKey].deferredRefreshAriaActiveItem();
  $authorSearchDropdown.dropdown('setting', 'values', values);
}
