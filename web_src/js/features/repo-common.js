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

export function initRepoClone() {
  // Quick start and repository home
  $('#repo-clone-ssh').on('click', function () {
    $('.clone-url').text($(this).data('link'));
    $('#repo-clone-url').val($(this).data('link'));
    $(this).addClass('primary');
    $('#repo-clone-https').removeClass('primary');
    localStorage.setItem('repo-clone-protocol', 'ssh');
  });
  $('#repo-clone-https').on('click', function () {
    $('.clone-url').text($(this).data('link'));
    $('#repo-clone-url').val($(this).data('link'));
    $(this).addClass('primary');
    if ($('#repo-clone-ssh').length > 0) {
      $('#repo-clone-ssh').removeClass('primary');
      localStorage.setItem('repo-clone-protocol', 'https');
    }
  });
  $('#repo-clone-url').on('click', function () {
    $(this).select();
  });
}

export function initRepoCommonBranchOrTagDropdown(selector) {
  $.find(selector).each(function () {
    const $dropdown = $(this);
    $dropdown.find('.reference.column').on('click', function () {
      $dropdown.find('.scrolling.reference-list-menu').hide();
      $.find($(this).data('target')).show();
      return false;
    });
  });
}

export function initRepoCommonFilterSearchDropdown(selector) {
  const $dropdown = $.find(selector);
  $dropdown.dropdown({
    fullTextSearch: true,
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice.data('url')) {
        window.location.href = $choice.data('url');
      }
    },
    message: {noResults: $dropdown.data('no-results')},
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
