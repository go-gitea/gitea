import $ from 'jquery';
import {svg} from '../svg.js';

function changeHash(hash) {
  if (window.history.pushState) {
    window.history.pushState(null, null, hash);
  } else {
    window.location.hash = hash;
  }
}

function selectRange($list, $select, $from) {
  $list.removeClass('active');

  // add hashchange to permalink
  const $issue = $('a.ref-in-new-issue');
  const $copyPermalink = $('a.copy-line-permalink');
  const $viewGitBlame = $('a.view_git_blame');

  const updateIssueHref = function (anchor) {
    if ($issue.length === 0) {
      return;
    }
    let href = $issue.attr('href');
    href = `${href.replace(/%23L\d+$|%23L\d+-L\d+$/, '')}%23${anchor}`;
    $issue.attr('href', href);
  };

  const updateViewGitBlameFragment = function (anchor) {
    if ($viewGitBlame.length === 0) {
      return;
    }
    let href = $viewGitBlame.attr('href');
    href = `${href.replace(/#L\d+$|#L\d+-L\d+$/, '')}`;
    if (anchor.length !== 0) {
      href = `${href}#${anchor}`;
    }
    $viewGitBlame.attr('href', href);
  };

  const updateCopyPermalinkHref = function(anchor) {
    if ($copyPermalink.length === 0) {
      return;
    }
    let link = $copyPermalink.attr('data-clipboard-text');
    link = `${link.replace(/#L\d+$|#L\d+-L\d+$/, '')}#${anchor}`;
    $copyPermalink.attr('data-clipboard-text', link);
  };

  if ($from) {
    let a = parseInt($select.attr('rel').slice(1));
    let b = parseInt($from.attr('rel').slice(1));
    let c;
    if (a !== b) {
      if (a > b) {
        c = a;
        a = b;
        b = c;
      }
      const classes = [];
      for (let i = a; i <= b; i++) {
        classes.push(`[rel=L${i}]`);
      }
      $list.filter(classes.join(',')).addClass('active');
      changeHash(`#L${a}-L${b}`);

      updateIssueHref(`L${a}-L${b}`);
      updateViewGitBlameFragment(`L${a}-L${b}`);
      updateCopyPermalinkHref(`L${a}-L${b}`);
      return;
    }
  }
  $select.addClass('active');
  changeHash(`#${$select.attr('rel')}`);

  updateIssueHref($select.attr('rel'));
  updateViewGitBlameFragment($select.attr('rel'));
  updateCopyPermalinkHref($select.attr('rel'));
}

function showLineButton() {
  if ($('.code-line-menu').length === 0) return;
  $('.code-line-button').remove();
  $('.code-view td.lines-code.active').closest('tr').find('td:eq(0)').first().prepend(
    $(`<button class="code-line-button">${svg('octicon-kebab-horizontal')}</button>`)
  );
  $('.code-line-menu').appendTo($('.code-view'));
  $('.code-line-button').popup({popup: $('.code-line-menu'), on: 'click'});
}

export function initRepoCodeView() {
  if ($('.code-view .lines-num').length > 0) {
    $(document).on('click', '.lines-num span', function (e) {
      const $select = $(this);
      let $list;
      if ($('div.blame').length) {
        $list = $('.code-view td.lines-code.blame-code');
      } else {
        $list = $('.code-view td.lines-code');
      }
      selectRange($list, $list.filter(`[rel=${$select.attr('id')}]`), (e.shiftKey ? $list.filter('.active').eq(0) : null));

      if (window.getSelection) {
        window.getSelection().removeAllRanges();
      } else {
        document.selection.empty();
      }

      // show code view menu marker (don't show in blame page)
      if ($('div.blame').length === 0) {
        showLineButton();
      }
    });

    $(window).on('hashchange', () => {
      let m = window.location.hash.match(/^#(L\d+)-(L\d+)$/);
      let $list;
      if ($('div.blame').length) {
        $list = $('.code-view td.lines-code.blame-code');
      } else {
        $list = $('.code-view td.lines-code');
      }
      let $first;
      if (m) {
        $first = $list.filter(`[rel=${m[1]}]`);
        selectRange($list, $first, $list.filter(`[rel=${m[2]}]`));

        // show code view menu marker (don't show in blame page)
        if ($('div.blame').length === 0) {
          showLineButton();
        }

        $('html, body').scrollTop($first.offset().top - 200);
        return;
      }
      m = window.location.hash.match(/^#(L|n)(\d+)$/);
      if (m) {
        $first = $list.filter(`[rel=L${m[2]}]`);
        selectRange($list, $first);

        // show code view menu marker (don't show in blame page)
        if ($('div.blame').length === 0) {
          showLineButton();
        }

        $('html, body').scrollTop($first.offset().top - 200);
      }
    }).trigger('hashchange');
  }
  $(document).on('click', '.fold-file', ({currentTarget}) => {
    const box = currentTarget.closest('.file-content');
    const folded = box.getAttribute('data-folded') !== 'true';
    currentTarget.innerHTML = svg(`octicon-chevron-${folded ? 'right' : 'down'}`, 18);
    box.setAttribute('data-folded', String(folded));
  });
  $(document).on('click', '.blob-excerpt', async ({currentTarget}) => {
    const url = currentTarget.getAttribute('data-url');
    const query = currentTarget.getAttribute('data-query');
    const anchor = currentTarget.getAttribute('data-anchor');
    if (!url) return;
    const blob = await $.get(`${url}?${query}&anchor=${anchor}`);
    currentTarget.closest('tr').outerHTML = blob;
  });
}
