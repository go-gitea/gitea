import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';

export function initCommonIssue() {
  const checkboxOperate = () => {
    if ($('.issue-checkbox input').is(':checked')) {
      $('#issue-filters').addClass('hide');
      $('#issue-actions').removeClass('hide');
      $('#issue-actions .six').prepend($('.issue-checkbox-all'));
    } else {
      $('#issue-filters').removeClass('hide');
      $('#issue-actions').addClass('hide');
      $('#issue-filters .six').prepend($('.issue-checkbox-all'));
    }
  };

  $('.issue-checkbox').on('click', checkboxOperate);

  $('.issue-checkbox-all').on('click', (e) => {
    const selected = $('.issue-checkbox input:checked');
    $('.issue-checkbox input:not(:checked)').prop('checked', 1);
    selected.prop('checked', 0);
    checkboxOperate(e);
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
