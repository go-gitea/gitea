import {debounce, throttle} from './func.ts';

test('debounce', () => {
  vi.useFakeTimers();
  const spy = vi.fn();
  const fn = debounce(spy, 10);
  fn();
  fn();
  fn();
  expect(spy).toHaveBeenCalledTimes(0);
  vi.advanceTimersByTime(30);
  expect(spy).toHaveBeenCalledTimes(1);
  vi.useRealTimers();
});

test('debounce leading', () => {
  vi.useFakeTimers();
  const spy = vi.fn();
  const fn = debounce(spy, 10, {leading: true, trailing: false});
  fn();
  expect(spy).toHaveBeenCalledTimes(1);
  fn();
  vi.advanceTimersByTime(30);
  expect(spy).toHaveBeenCalledTimes(1);
  vi.useRealTimers();
});

test('debounce result', async () => {
  vi.useFakeTimers();
  const fn = debounce((value: number) => value * 2, 10);
  const first = fn(1);
  const second = fn(2);
  vi.advanceTimersByTime(10);
  expect(await first).toEqual(4); // both calls collapse into the last one
  expect(await second).toEqual(4);
  vi.useRealTimers();
});

test('debounce cancel', () => {
  vi.useFakeTimers();
  const spy = vi.fn();
  const fn = debounce(spy, 10);
  fn();
  fn.cancel();
  vi.advanceTimersByTime(30);
  expect(spy).toHaveBeenCalledTimes(0);
  vi.useRealTimers();
});

test('throttle', () => {
  vi.useFakeTimers();
  const spy = vi.fn();
  const fn = throttle(spy, 10);
  fn();
  fn();
  fn();
  expect(spy).toHaveBeenCalledTimes(1); // leading
  vi.advanceTimersByTime(30);
  expect(spy).toHaveBeenCalledTimes(2); // plus one trailing for the collapsed rest
  vi.useRealTimers();
});

test('throttle trailing only', () => {
  vi.useFakeTimers();
  const spy = vi.fn();
  const fn = throttle(spy, 10, {leading: false});
  fn();
  fn();
  fn();
  expect(spy).toHaveBeenCalledTimes(0);
  vi.advanceTimersByTime(30);
  expect(spy).toHaveBeenCalledTimes(1);
  vi.useRealTimers();
});
