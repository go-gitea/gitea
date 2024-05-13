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

function isBlame() {
  return Boolean(document.querySelector('div.blame'));
}

function getLineEls() {
  return document.querySelectorAll(`.code-view td.lines-code${isBlame() ? '.blame-code' : ''}`);
}

function selectRange($linesEls, $selectionEndEl, $selectionStartEls) {
  for (const el of $linesEls) {
    el.closest('tr').classList.remove('active');
  }

  // add hashchange to permalink
  const refInNewIssue = document.querySelector('a.ref-in-new-issue');
  const copyPermalink = document.querySelector('a.copy-line-permalink');
  const viewGitBlame = document.querySelector('a.view_git_blame');

  const updateIssueHref = function (anchor) {
    if (!refInNewIssue) return;
    const urlIssueNew = refInNewIssue.getAttribute('data-url-issue-new');
    const urlParamBodyLink = refInNewIssue.getAttribute('data-url-param-body-link');
    const issueContent = `${toAbsoluteUrl(urlParamBodyLink)}#${anchor}`; // the default content for issue body
    refInNewIssue.setAttribute('href', `${urlIssueNew}?body=${encodeURIComponent(issueContent)}`);
  };

  const updateViewGitBlameFragment = function (anchor) {
    if (!viewGitBlame) return;
    let href = viewGitBlame.getAttribute('href');
    href = `${href.replace(/#L\d+$|#L\d+-L\d+$/, '')}`;
    if (anchor.length !== 0) {
      href = `${href}#${anchor}`;
    }
    viewGitBlame.setAttribute('href', href);
  };

  const updateCopyPermalinkUrl = function (anchor) {
    if (!copyPermalink) return;
    let link = copyPermalink.getAttribute('data-url');
    link = `${link.replace(/#L\d+$|#L\d+-L\d+$/, '')}#${anchor}`;
    copyPermalink.setAttribute('data-url', link);
  };

  if ($selectionStartEls) {
    let a = parseInt($selectionEndEl[0].getAttribute('rel').slice(1));
    let b = parseInt($selectionStartEls[0].getAttribute('rel').slice(1));
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
      $linesEls.filter(classes.join(',')).each(function () {
        this.closest('tr').classList.add('active');
      });
      changeHash(`#L${a}-L${b}`);

      updateIssueHref(`L${a}-L${b}`);
      updateViewGitBlameFragment(`L${a}-L${b}`);
      updateCopyPermalinkUrl(`L${a}-L${b}`);
      return;
    }
  }
  $selectionEndEl[0].closest('tr').classList.add('active');
  changeHash(`#${$selectionEndEl[0].getAttribute('rel')}`);

  updateIssueHref($selectionEndEl[0].getAttribute('rel'));
  updateViewGitBlameFragment($selectionEndEl[0].getAttribute('rel'));
  updateCopyPermalinkUrl($selectionEndEl[0].getAttribute('rel'));
}

function showLineButton() {
  const menu = document.querySelector('.code-line-menu');
  if (!menu) return;

  // remove all other line buttons
  for (const el of document.querySelectorAll('.code-line-button')) {
    el.remove();
  }

  // find active row and add button
  const tr = document.querySelector('.code-view tr.active');
  const td = tr.querySelector('td.lines-num');
  const btn = document.createElement('button');
  btn.classList.add('code-line-button', 'ui', 'basic', 'button');
  btn.innerHTML = svg('octicon-kebab-horizontal');
  td.prepend(btn);

  // put a copy of the menu back into DOM for the next click
  btn.closest('.code-view').append(menu.cloneNode(true));

  createTippy(btn, {
    theme: 'menu',
    trigger: 'click',
    hideOnClick: true,
    content: menu,
    placement: 'right-start',
    interactive: true,
    onShow: (tippy) => {
      tippy.popper.addEventListener('click', () => {
        tippy.hide();
      }, {once: true});
    },
  });
}

export function initRepoCodeView() {
  if ($('.code-view .lines-num').length > 0) {
    $(document).on('click', '.lines-num span', function (e) {
      const linesEls = getLineEls();
      const selectedEls = Array.from(linesEls).filter((el) => {
        return el.matches(`[rel=${this.getAttribute('id')}]`);
      });

      let from;
      if (e.shiftKey) {
        from = Array.from(linesEls).filter((el) => {
          return el.closest('tr').classList.contains('active');
        });
      }
      selectRange($(linesEls), $(selectedEls), from ? $(from) : null);

      if (window.getSelection) {
        window.getSelection().removeAllRanges();
      } else {
        document.selection.empty();
      }

      showLineButton();
    });

    $(window).on('hashchange', () => {
      let m = window.location.hash.match(rangeAnchorRegex);
      const $linesEls = $(getLineEls());
      let $first;
      if (m) {
        $first = $linesEls.filter(`[rel=${m[1]}]`);
        if ($first.length) {
          selectRange($linesEls, $first, $linesEls.filter(`[rel=${m[2]}]`));

          // show code view menu marker (don't show in blame page)
          if (!isBlame()) {
            showLineButton();
          }

          $('html, body').scrollTop($first.offset().top - 200);
          return;
        }
      }
      m = window.location.hash.match(singleAnchorRegex);
      if (m) {
        $first = $linesEls.filter(`[rel=L${m[2]}]`);
        if ($first.length) {
          selectRange($linesEls, $first);

          // show code view menu marker (don't show in blame page)
          if (!isBlame()) {
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
  $(document).on('click', '.copy-line-permalink', async ({currentTarget}) => {
    await clippie(toAbsoluteUrl(currentTarget.getAttribute('data-url')));
  });
}
