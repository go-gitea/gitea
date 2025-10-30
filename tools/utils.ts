export function isTruthy<T>(value: T): value is T extends false | '' | 0 | null | undefined ? never : T { // eslint-disable-line unicorn/prefer-native-coercion-functions
  return Boolean(value);
}
