import $ from 'jquery';

export function initOrgTimes() {
  // Comfort function to auto-open 2nd date picker of range picker, modeled on Formantic UI's behaviour
  $('#rangefrom').on('change', () => {
    document.getElementById('rangeto').showPicker();
  });
}
