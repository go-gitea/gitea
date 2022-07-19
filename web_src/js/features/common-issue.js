import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';

export function initCommonIssue() {
  $('.issue-checkbox').on('click', () => {
    const numChecked = $('.issue-checkbox').children('input:checked').length;
    if (numChecked > 0) {
      $('#issue-filters').addClass('hide');
      $('#issue-actions').removeClass('hide');
    } else {
      $('#issue-filters').removeClass('hide');
      $('#issue-actions').addClass('hide');
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
