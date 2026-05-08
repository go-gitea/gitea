import {toggleTasklistCheckbox} from './tasklist.ts';

test('toggleTasklistCheckbox', () => {
  expect(toggleTasklistCheckbox('- [ ] task', 3, true)).toEqual('- [x] task');
  expect(toggleTasklistCheckbox('- [x] task', 3, false)).toEqual('- [ ] task');
  expect(toggleTasklistCheckbox('- [ ] task', 0, true)).toBeNull();
  expect(toggleTasklistCheckbox('- [ ] task', 99, true)).toBeNull();
  expect(toggleTasklistCheckbox('😀 - [ ] task', 8, true)).toEqual('😀 - [x] task');
});
