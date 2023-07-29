import $ from 'jquery';
import {showInfoToast, showWarningToast, showErrorToast} from '../modules/toast.js';

document.getElementById('info-toast')?.addEventListener('click', () => {
  showInfoToast('success 😀');
});
document.getElementById('warning-toast')?.addEventListener('click', () => {
  showWarningToast('warning 😐');
});
document.getElementById('error-toast')?.addEventListener('click', () => {
  showErrorToast('error 🙁');
});

const $buttons = $('#devtest-button-samples').find('button.ui');

const $buttonStyles = $('input[name*="button-style"]');
$buttonStyles.on('click', () => $buttonStyles.map((_, el) => $buttons.toggleClass(el.value, el.checked)));

const $buttonStates = $('input[name*="button-state"]');
$buttonStates.on('click', () => $buttonStates.map((_, el) => $buttons.prop(el.value, el.checked)));

for (const el of $('.ui.modal')) {
  const $btn = $('<button>').text(`Show ${el.id}`).on('click', () => {
    $(el).modal({onApprove() {alert('confirmed')}}).modal('show');
  });
  $('.modal-buttons').append($btn);
}
