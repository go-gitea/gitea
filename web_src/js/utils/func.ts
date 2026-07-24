import type {TimeoutId} from '../types.ts';

/** Options for `debounce` */
export type DebounceOpts = {
  /** Invoke on the leading edge of the wait period. Default: `false` */
  leading?: boolean,
  /** Invoke on the trailing edge of the wait period. Default: `true` */
  trailing?: boolean,
};

/** Options for `throttle` */
export type ThrottleOpts = {
  /** Invoke on the leading edge of the interval. Default: `true` */
  leading?: boolean,
  /** Invoke on the trailing edge of the interval. Default: `true` */
  trailing?: boolean,
};

/** A debounced or throttled function. Calls collapsed into one invocation settle with its result, dropped calls never settle. */
export type TimedFunction<T extends (...args: Array<any>) => any> = ((...args: Parameters<T>) => Promise<Awaited<ReturnType<T>>>) & {
  /** Drop the pending invocation */
  cancel: () => void,
};

function createTimed<T extends (...args: Array<any>) => any>(func: T, wait: number, leading: boolean, trailing: boolean, isThrottle: boolean): TimedFunction<T> {
  let timer: TimeoutId | null = null;
  let pendingArgs: Parameters<T> | null = null;
  let resolvers: Array<{resolve: (value: any) => void, reject: (reason: any) => void}> = [];

  const invoke = async (args: Parameters<T>): Promise<void> => {
    const settling = resolvers;
    resolvers = [];
    try {
      const value = await func(...args);
      for (const {resolve} of settling) resolve(value);
    } catch (err) {
      for (const {reject} of settling) reject(err);
    }
  };

  const onTimer = (): void => {
    timer = null;
    const args = pendingArgs;
    pendingArgs = null;
    if (trailing && args) {
      invoke(args);
      if (isThrottle) timer = setTimeout(onTimer, wait); // keep the window open so a burst stays rate-limited
    } else {
      resolvers = [];
    }
  };

  const cancel = (): void => {
    if (timer) clearTimeout(timer);
    timer = null;
    pendingArgs = null;
    resolvers = [];
  };

  const wrapper = (...args: Parameters<T>): Promise<Awaited<ReturnType<T>>> => {
    const promise = new Promise<Awaited<ReturnType<T>>>((resolve, reject) => {
      resolvers.push({resolve, reject});
    });
    const isLeadingCall = leading && timer === null;
    if (!isThrottle && timer) { clearTimeout(timer); timer = null } // debounce restarts the wait on every call, throttle does not
    if (isLeadingCall) invoke(args); else pendingArgs = args;
    if (timer === null) timer = setTimeout(onTimer, wait);
    return promise;
  };

  return Object.assign(wrapper, {cancel});
}

/** Debounce a function, delaying invocation until `wait` milliseconds have passed without another call */
export function debounce<T extends (...args: Array<any>) => any>(func: T, wait: number, {leading = false, trailing = true}: DebounceOpts = {}): TimedFunction<T> {
  return createTimed(func, wait, leading, trailing, false);
}

/** Throttle a function to invoke at most once per `interval` milliseconds */
export function throttle<T extends (...args: Array<any>) => any>(func: T, interval: number, {leading = true, trailing = true}: ThrottleOpts = {}): TimedFunction<T> {
  return createTimed(func, interval, leading, trailing, true);
}
