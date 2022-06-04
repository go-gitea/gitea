import $ from 'jquery';

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
  const defaultGitProtocol = 'https'; // ssh or https

  const $repoCloneSsh = $('#repo-clone-ssh');
  const $repoCloneHttps = $('#repo-clone-https');
  const $inputLink = $('#repo-clone-url');

  if ((!$repoCloneSsh.length && !$repoCloneHttps.length) || !$inputLink.length) {
    return;
  }

  const updateUi = () => {
    let isSSH = (localStorage.getItem('repo-clone-protocol') || defaultGitProtocol) === 'ssh';
    // there must be at least one clone button (by context/repo.go). if no ssh, then there must be https.
    if (isSSH && $repoCloneSsh.length === 0) {
      isSSH = false;
    } else if (!isSSH && $repoCloneHttps.length === 0) {
      isSSH = true;
    }
    const cloneLink = (isSSH ? $repoCloneSsh : $repoCloneHttps).attr('data-link');
    $inputLink.val(cloneLink);
    if (isSSH) {
      $repoCloneSsh.addClass('primary');
      $repoCloneHttps.removeClass('primary');
    } else {
      $repoCloneSsh.removeClass('primary');
      $repoCloneHttps.addClass('primary');
    }
    // the empty repo guide
    $('.quickstart .empty-repo-guide .clone-url').text(cloneLink);
  };
  updateUi();

  setTimeout(() => {
    // restore animation after first init
    $repoCloneSsh.removeClass('no-transition');
    $repoCloneHttps.removeClass('no-transition');
  }, 100);

  $repoCloneSsh.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    updateUi();
  });
  $repoCloneHttps.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
    updateUi();
  });

  $inputLink.on('click', () => {
    $inputLink.select();
  });
}

export function initRepoCommonBranchOrTagDropdown(selector) {
  $(selector).each(function () {
    const $dropdown = $(this);
    $dropdown.find('.reference.column').on('click', function () {
      $dropdown.find('.scrolling.reference-list-menu').hide();
      $($(this).data('target')).show();
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
      $('.language-stats-details, .repository-menu').slideToggle();
    });
  }
}
