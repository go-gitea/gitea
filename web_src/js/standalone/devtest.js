import {showInfoToast, showWarningToast, showErrorToast} from '../modules/toast.js';

document.getElementById('info-toast').addEventListener('click', () => {
  showInfoToast('success 😀');
});
document.getElementById('warning-toast').addEventListener('click', () => {
  showWarningToast('warning 😐');
});
document.getElementById('error-toast').addEventListener('click', () => {
  showErrorToast('error 🙁');
});
