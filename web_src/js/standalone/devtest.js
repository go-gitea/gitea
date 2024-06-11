import {showInfoToast, showWarningToast, showErrorToast} from '../modules/toast.js';

document.querySelector('#info-toast').addEventListener('click', () => {
  showInfoToast('success 😀');
});
document.querySelector('#warning-toast').addEventListener('click', () => {
  showWarningToast('warning 😐');
});
document.querySelector('#error-toast').addEventListener('click', () => {
  showErrorToast('error 🙁');
});
