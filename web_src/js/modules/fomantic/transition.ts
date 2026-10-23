export function initFomanticTransition() {
  // stand-in for removed transition module
  $.fn.transition = function (arg0: any) {
    if (arg0 === 'is supported') return true;
    if (arg0 === 'is animating') return false;
    if (arg0 === 'is inward') return false;
    if (arg0 === 'is outward') return false;

    let argObj: Record<string, any>;
    if (typeof arg0 === 'string') {
      if (arg0 === 'stop all') return this;
      argObj = {animation: arg0};
    } else if (typeof arg0 === 'object') {
      argObj = arg0;
    } else {
      throw new Error(`invalid argument: ${arg0}`);
    }

    const isAnimationIn = argObj.animation?.startsWith('show') || argObj.animation?.endsWith(' in');
    const isAnimationOut = argObj.animation?.startsWith('hide') || argObj.animation?.endsWith(' out');
    this.each((_, el) => {
      let toShow = isAnimationIn;
      if (!isAnimationIn && !isAnimationOut) {
        // If the animation is not in/out, then it must be a toggle animation.
        // Fomantic uses computed styles to check "visibility", but to avoid unnecessary arguments, here it only checks the class.
        toShow = this.hasClass('hidden'); // maybe it could also check "!this.hasClass('visible')", leave it to the future until there is a real problem.
      }
      argObj.onStart?.call(el);
      if (toShow) {
        el.classList.remove('hidden');
        el.classList.add('visible', 'transition');
        if (argObj.displayType) el.style.setProperty('display', argObj.displayType, 'important');
      } else {
        el.classList.add('hidden');
        el.classList.remove('visible'); // don't remove the transition class because the Fomantic animation style is `.hidden.transition`.
        el.style.removeProperty('display');
      }
      argObj.onComplete?.call(el);
    });
    return this;
  };
}
