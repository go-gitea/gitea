import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';

export function initCommonIssue() {
  const $issueSelectAllWrapper = $('.issue-checkbox-all');
  const $issueSelectAll = $('.issue-checkbox-all input');
  const $issueCheckboxes = $('.issue-checkbox input');

  const syncIssueSelectionState = () => {
    const $checked = $issueCheckboxes.filter(':checked');
    const anyChecked = $checked.length !== 0;
    const allChecked = anyChecked && $checked.length === $issueCheckboxes.length;

    if (allChecked) {
      $issueSelectAll.prop({'checked': true, 'indeterminate': false});
    } else if (anyChecked) {
      $issueSelectAll.prop({'checked': false, 'indeterminate': true});
    } else {
      $issueSelectAll.prop({'checked': false, 'indeterminate': false});
    }
    // if any issue is selected, show the action panel, otherwise show the filter panel
    $('#issue-filters').toggle(!anyChecked);
    $('#issue-actions').toggle(anyChecked);
    // there are two panels but only one select-all checkbox, so move the checkbox to the visible panel
    $('#issue-filters, #issue-actions').filter(':visible').find('.column:first').prepend($issueSelectAllWrapper);
  };

  $issueCheckboxes.on('change', syncIssueSelectionState);

  $issueSelectAll.on('change', () => {
    $issueCheckboxes.prop('checked', $issueSelectAll.is(':checked'));
    syncIssueSelectionState();
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
  // checked after reload trigger checked event, if checkboxes are checked on load
  $('.issue-checkbox input[type="checkbox"]:checked').first().each((_, e) => {
    e.checked = false;
    $(e).trigger('click');
  });
}
