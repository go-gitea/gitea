import $ from 'jquery';
import {svg} from '../svg.js';
import {invertFileFolding} from './file-fold.js';
import {createTippy} from '../modules/tippy.js';
import {clippie} from 'clippie';
import {toAbsoluteUrl} from '../utils.js';

export const singleAnchorRegex = /^#(L|n)([1-9][0-9]*)$/;
export const rangeAnchorRegex = /^#(L[1-9][0-9]*)-(L[1-9][0-9]*)$/;

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
  const $refInNewIssue = $('a.ref-in-new-issue');
  const $copyPermalink = $('a.copy-line-permalink');
  const $viewGitBlame = $('a.view_git_blame');

  const updateIssueHref = function (anchor) {
    if ($refInNewIssue.length === 0) {
      return;
    }
    const urlIssueNew = $refInNewIssue.attr('data-url-issue-new');
    const urlParamBodyLink = $refInNewIssue.attr('data-url-param-body-link');
    const issueContent = `${toAbsoluteUrl(urlParamBodyLink)}#${anchor}`; // the default content for issue body
    $refInNewIssue.attr('href', `${urlIssueNew}?body=${encodeURIComponent(issueContent)}`);
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

  const updateCopyPermalinkUrl = function(anchor) {
    if ($copyPermalink.length === 0) {
      return;
    }
    let link = $copyPermalink.attr('data-url');
    link = `${link.replace(/#L\d+$|#L\d+-L\d+$/, '')}#${anchor}`;
    $copyPermalink.attr('data-url', link);
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
      updateCopyPermalinkUrl(`L${a}-L${b}`);
      return;
    }
  }
  $select.addClass('active');
  changeHash(`#${$select.attr('rel')}`);

  updateIssueHref($select.attr('rel'));
  updateViewGitBlameFragment($select.attr('rel'));
  updateCopyPermalinkUrl($select.attr('rel'));
}

function showLineButton() {
  const menu = document.querySelector('.code-line-menu');
  if (!menu) return;

  // remove all other line buttons
  for (const el of document.querySelectorAll('.code-line-button')) {
    el.remove();
  }

  // find active row and add button
  const tr = document.querySelector('.code-view td.lines-code.active').closest('tr');
  const td = tr.querySelector('td');
  const btn = document.createElement('button');
  btn.classList.add('code-line-button');
  btn.innerHTML = svg('octicon-kebab-horizontal');
  td.prepend(btn);

  // put a copy of the menu back into DOM for the next click
  btn.closest('.code-view').appendChild(menu.cloneNode(true));

  createTippy(btn, {
    trigger: 'click',
    content: menu,
    placement: 'right-start',
    role: 'menu',
    interactive: 'true',
  });
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
      let m = window.location.hash.match(rangeAnchorRegex);
      let $list;
      if ($('div.blame').length) {
        $list = $('.code-view td.lines-code.blame-code');
      } else {
        $list = $('.code-view td.lines-code');
      }
      let $first;
      if (m) {
        $first = $list.filter(`[rel=${m[1]}]`);
        if ($first.length) {
          selectRange($list, $first, $list.filter(`[rel=${m[2]}]`));

          // show code view menu marker (don't show in blame page)
          if ($('div.blame').length === 0) {
            showLineButton();
          }

          $('html, body').scrollTop($first.offset().top - 200);
          return;
        }
      }
      m = window.location.hash.match(singleAnchorRegex);
      if (m) {
        $first = $list.filter(`[rel=L${m[2]}]`);
        if ($first.length) {
          selectRange($list, $first);

          // show code view menu marker (don't show in blame page)
          if ($('div.blame').length === 0) {
            showLineButton();
          }

          $('html, body').scrollTop($first.offset().top - 200);
        }
      }
    }).trigger('hashchange');
  }
  $(document).on('click', '.fold-file', ({currentTarget}) => {
    invertFileFolding(currentTarget.closest('.file-content'), currentTarget);
  });
  $(document).on('click', '.blob-excerpt', async ({currentTarget}) => {
    const url = currentTarget.getAttribute('data-url');
    const query = currentTarget.getAttribute('data-query');
    const anchor = currentTarget.getAttribute('data-anchor');
    if (!url) return;
    const blob = await $.get(`${url}?${query}&anchor=${anchor}`);
    currentTarget.closest('tr').outerHTML = blob;
  });
  $(document).on('click', '.copy-line-permalink', async (e) => {
    const success = await clippie(toAbsoluteUrl(e.currentTarget.getAttribute('data-url')));
    if (!success) return;
    document.querySelector('.code-line-button')?._tippy?.hide();
  });
}
