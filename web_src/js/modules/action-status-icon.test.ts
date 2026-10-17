import {getActionStatusIcon} from './action-status-icon.ts';

test('getActionStatusIcon', () => {
  expect(getActionStatusIcon('success')).toEqual({name: 'octicon-check', colorClass: 'text-green'});
  expect(getActionStatusIcon('success', 'circle-fill')).toEqual({name: 'octicon-check-circle-fill', colorClass: 'text-green'});
  expect(getActionStatusIcon('running')).toEqual({name: 'gitea-running', colorClass: 'text-yellow'});
  expect(getActionStatusIcon('failure', 'circle-fill')).toEqual({name: 'octicon-x-circle-fill', colorClass: 'text-red'});
  expect(getActionStatusIcon('cancelled')).toEqual({name: 'octicon-stop', colorClass: 'text-text-light'});
});
