export function weakRefClass() {
  const weakMap = new WeakMap();
  return class {
    constructor(target: any) {
      weakMap.set(this, target);
    }
    deref() {
      return weakMap.get(this);
    }
  };
}

if (!window.WeakRef) {
  window.WeakRef = weakRefClass() as any;
}
