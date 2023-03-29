import $ from 'jquery';
import {hideElem, showElem, toggleElem, setAttributes} from '../utils/dom.js';

const {appSubUrl, csrfToken} = window.config;

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

export async function initPostersDropdown() {
  console.log('fetch posters data');
  const $autherSearch = document.getElementById('author-search');
  if (!$autherSearch) {
    return;
  }
  const url = $autherSearch.getAttribute('data-url');
  const posterID = $autherSearch.getAttribute('data-poster-id');
  const isShowFullName = $autherSearch.getAttribute('data-show-fullname');
  const posterGeneralHref = $autherSearch.getAttribute('data-general-poster-href');
  const posterList = document.querySelector('.poster-list');
  const res = await fetch(url, {
    method: 'GET'
  });
  const postersJson = await res.json();
  console.log(res);
  console.log(postersJson);
  if (!postersJson) {
    $autherSearch.classList.add('disabled');
  } else {
    $('.poster-list').find('.poster-item').remove();
    for (let i = 0; i < postersJson.length; i++) {
      const {id, avatar_url, username, full_name} = postersJson[i];
      const $a = document.createElement('a');
      setAttributes($a, {
        'class': `item gt-df poster-item${posterID === id ? ' active selected' : ''}`,
        'href': `${posterGeneralHref}${id}`,
      });
      const $img = document.createElement('img');
      setAttributes($img, {
        'class': 'ui avatar gt-vm',
        'src': avatar_url,
        'title': username,
        'width': '28',
        'height': '28'
      });
      $a.appendChild($img);
      const $span = document.createElement('span');
      setAttributes($span, {'class': 'gt-ellipsis'});
      $span.innerHTML = username;
      if (isShowFullName === 'true') {
        const $spanInner = document.createElement('span');
        setAttributes($spanInner, {'class': 'search-fullname'});
        $spanInner.innerHTML = full_name;
        $span.appendChild($spanInner);
      }
      $a.appendChild($span);
      posterList.appendChild($a);
    }
    delete $('#author-search')[0]['_giteaAriaPatchDropdown'];
    $('#author-search').dropdown();
  }
}

export async function initPostersDropdownTest() {
  console.log('fetch posters data 1');
  const $autherSearch = document.getElementById('author-search-1');
  if (!$autherSearch) {
    return;
  }
  const url = $autherSearch.getAttribute('data-url');
  const posterID = $autherSearch.getAttribute('data-poster-id');
  const isShowFullName = $autherSearch.getAttribute('data-show-fullname');
  const posterGeneralHref = $autherSearch.getAttribute('data-general-poster-href');
  const posterList = document.querySelector('.poster-list');
  console.log('url', `${appSubUrl}${url}`)
  $('#author-search-1').dropdown({
    apiSettings: {
      fullTextSearch: 'exact',
      saveRemoteData: false,
      // this url parses query server side and returns filtered results
      url: `${appSubUrl}${url}`,
      onResponse(response) {
        console.log(response)
        const formattedResponse = {success: true, results: []};
        // Parse the response from the api to work with our dropdown
        $.each(response, (_, poster) => {
          const {id, avatar_url, username, full_name} = poster;
          console.log(poster)
          formattedResponse.results.push({
            name: `<a class="item gt-df${posterID === id ? ' active selected' : ''}" href="${posterGeneralHref}${id}">
              <img class="ui avatar gt-vm" src="${avatar_url}" title="${username}" width="28" height="28">
              <span class="gt-ellipsis">${username}${isShowFullName === 'true' ? `<span class="search-fullname"> ${full_name}</span>`: ''}</span>
            </a>`,
            value: id,
          });
        });
        return formattedResponse;
      },
      cache: false,
    },
    // fields: {
    // the remote api has a different structure than expected, which we can adjust
    // remoteValues: 'item'
    // }
  });
}
