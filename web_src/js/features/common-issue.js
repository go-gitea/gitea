import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';

export function initCommonIssue() {
  $('.issue-checkbox,.issue-checkbox-all').on('click', (e) => {
    const issuecheckbox = $('.issue-checkbox input');
    if (e.currentTarget.className.includes('issue-checkbox-all')) {
      if ($('.issue-checkbox-all input').prop('checked')) {
        const selected = $('.issue-checkbox input:checked');
        $('.issue-checkbox input:not(:checked)').prop('checked', 1);
        selected.prop('checked', 0);
      } else {
        $('.issue-checkbox input:checked').prop('checked', 0);
      }
    }
    if (e.shiftKey && window.config.checkboxfirst !== undefined) {
      for (let i = window.config.checkboxfirst + 1, j = issuecheckbox.index($(e.currentTarget).find('input')); i < j; i++) {
        issuecheckbox[i].checked = 1;
      }
      delete window.config.checkboxfirst;
    } else {
      window.config.checkboxfirst = issuecheckbox.index($(e.currentTarget).find('input'));
    }
    if (issuecheckbox.is(':checked')) {
      $('#issue-filters').addClass('hide');
      $('#issue-actions').removeClass('hide');
      $('#issue-actions .six').prepend($('.issue-checkbox-all'));
    } else {
      $('#issue-filters').removeClass('hide');
      $('#issue-actions').addClass('hide');
      $('#issue-filters .six').prepend($('.issue-checkbox-all'));
    }
  });

  $('.issue-action').on('click', async function () {
    let action = this.getAttribute('data-action');
    let elementId = this.getAttribute('data-element-id');
    const url = this.getAttribute('data-url');
    const issueIDs = $('.issue-checkbox').children('input:checked').map((_, el) => {
      return el.getAttribute('data-issue-id');
    }).get().join(',');
    if (elementId === '0' && url.slice(-9) === '/assignee') {
      elementId = '';
      action = 'clear';
    }
    updateIssuesMeta(
      url,
      action,
      issueIDs,
      elementId
    ).then(() => {
      // NOTICE: This reset of checkbox state targets Firefox caching behaviour, as the
      // checkboxes stay checked after reload
      if (action === 'close' || action === 'open') {
        // uncheck all checkboxes
        $('.issue-checkbox input[type="checkbox"]').each((_, e) => { e.checked = false });
      }
      window.location.reload();
    });
  });

  // NOTICE: This event trigger targets Firefox caching behaviour, as the checkboxes stay
  // checked after reload trigger ckecked event, if checkboxes are checked on load
  $('.issue-checkbox input[type="checkbox"]:checked').first().each((_, e) => {
    e.checked = false;
    $(e).trigger('click');
  });
}
