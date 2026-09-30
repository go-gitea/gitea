import {getActionStatusIcon} from './action-status-icon.ts';

test('getActionStatusIcon', () => {
  expect(getActionStatusIcon('success')).toEqual({name: 'octicon-check', colorClass: 'tw-text-green'});
  expect(getActionStatusIcon('success', 'circle-fill')).toEqual({name: 'octicon-check-circle-fill', colorClass: 'tw-text-green'});
  expect(getActionStatusIcon('running')).toEqual({name: 'gitea-running', colorClass: 'tw-text-yellow'});
  expect(getActionStatusIcon('failure', 'circle-fill')).toEqual({name: 'octicon-x-circle-fill', colorClass: 'tw-text-red'});
  expect(getActionStatusIcon('cancelled')).toEqual({name: 'octicon-stop', colorClass: 'tw-text-text-light'});
});
