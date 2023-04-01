import $ from 'jquery';
import {updateIssuesMeta} from './repo-issue.js';
import {toggleElem} from '../utils/dom.js';

export function initCommonIssue() {
  const $issueSelectAll = $('.issue-checkbox-all');
  const $issueCheckboxes = $('.issue-checkbox');

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
    toggleElem($('#issue-filters'), !anyChecked);
    toggleElem($('#issue-actions'), anyChecked);
    // there are two panels but only one select-all checkbox, so move the checkbox to the visible panel
    $('#issue-filters, #issue-actions').filter(':visible').find('.column:first').prepend($issueSelectAll);
  };

  $issueCheckboxes.on('change', syncIssueSelectionState);

  $issueSelectAll.on('change', () => {
    $issueCheckboxes.prop('checked', $issueSelectAll.is(':checked'));
    syncIssueSelectionState();
  });

  $('.issue-action').on('click', async function (e) {
    e.preventDefault();
    let action = this.getAttribute('data-action');
    let elementId = this.getAttribute('data-element-id');
    const url = this.getAttribute('data-url');
    const issueIDs = $('.issue-checkbox:checked').map((_, el) => {
      return el.getAttribute('data-issue-id');
    }).get().join(',');
    if (elementId === '0' && url.slice(-9) === '/assignee') {
      elementId = '';
      action = 'clear';
    }
    if (action === 'toggle' && e.altKey) {
      action = 'toggle-alt';
    }
    updateIssuesMeta(
      url,
      action,
      issueIDs,
      elementId
    ).then(() => {
      window.location.reload();
    });
  });
}
