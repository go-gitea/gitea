import {userEvent} from 'vitest/browser';
import {createFloatingElement, getFloatingElement, initGlobalTooltips} from './floating.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import '../../css/modules/floating.css';

function makeTarget(): HTMLElement {
  const el = document.createElement('button');
  document.body.append(el);
  return el;
}

describe('floating', () => {
  beforeEach(() => { document.body.replaceChildren() });
  afterEach(() => { vi.useRealTimers() });

  test('register', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'manual'});
    expect(getFloatingElement(target)).toBe(instance);
    instance.destroy();
    expect(getFloatingElement(target)).toBeNull();
  });

  test('show/hide', () => {
    const instance = createFloatingElement(makeTarget(), {content: 'x', trigger: 'manual'});
    instance.show();
    expect(instance.state.isShown).toBe(true);
    expect(instance.element.parentElement).toBe(document.body);
    instance.hide();
    expect(instance.state.isShown).toBe(false);
    expect(instance.element.parentElement).toBeNull();
    expect(instance.reference.getAttribute('aria-controls')).toBe(instance.element.id);
    instance.destroy();
  });

  test('setContent', () => {
    const instance = createFloatingElement(makeTarget(), {content: 'a', trigger: 'manual'});
    instance.setContent('b');
    expect(instance.element.querySelector('.floating-content')!.textContent).toBe('b');
    instance.destroy();
  });

  test('hover delay', () => {
    vi.useFakeTimers({toFake: ['setTimeout', 'clearTimeout']});
    const target = makeTarget();
    const instance = createFloatingElement(target, {delay: [40, 0]});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    expect(instance.state.isShown).toBe(false);
    vi.advanceTimersByTime(40);
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('click toggles', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'click'});
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('focus click', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'focus click'});
    target.dispatchEvent(new FocusEvent('focus'));
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    instance.destroy();
  });

  test('interactive hover', async () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {interactive: true});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    instance.element.dispatchEvent(new MouseEvent('mouseenter'));
    target.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(true);
    instance.element.dispatchEvent(new MouseEvent('mouseleave'));
    await vi.waitFor(() => expect(instance.state.isShown).toBe(false));
    instance.destroy();
  });

  test('tooltip dedupe', () => {
    const a = createFloatingElement(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    const b = createFloatingElement(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    a.show();
    b.show();
    expect(a.state.isShown).toBe(false);
    expect(b.state.isShown).toBe(true);
    a.destroy();
    b.destroy();
  });

  test('createFloatingElement handles keys in menus only', async () => {
    const menuButton = createElementFromHTML('<button>menu</button>');
    const panelButton = createElementFromHTML('<button>panel</button>');
    document.body.append(menuButton, panelButton);

    const clicked: Array<string> = [];
    const menuContent = createElementFromHTML('<div><button class="item">a</button><button class="item">b</button></div>');
    for (const item of menuContent.querySelectorAll('.item')) item.addEventListener('click', () => { clicked.push(item.textContent) });
    const menu = createFloatingElement(menuButton, {content: menuContent, theme: 'menu', trigger: 'manual', interactive: true});
    menu.show();
    await userEvent.keyboard('{ArrowDown}{ArrowDown} {Enter}');
    expect(clicked).toEqual(['b', 'b']);
    await userEvent.keyboard('{Escape}');
    expect(menu.state.isShown).toBe(false);
    expect(document.activeElement).toBe(menuButton);

    menu.show();
    const panel = createFloatingElement(panelButton, {content: createElementFromHTML('<div><textarea></textarea></div>'), trigger: 'manual', interactive: true});
    panel.show();
    const textarea = panel.element.querySelector('textarea')!;
    textarea.focus();
    await userEvent.keyboard('a b{Enter}{ArrowUp}c{Escape}');
    expect(textarea.value).toEqual('ca b\n');
    expect(menu.state.isShown).toBe(true);
    expect(panel.state.isShown).toBe(true);

    menu.destroy();
    panel.destroy();
    menuButton.remove();
    panelButton.remove();
  });

  test('limits oversized popup content to the viewport with scrollbars', async () => {
    const target = makeTarget();
    target.style.cssText = 'position: fixed; left: 50%; top: 50%';
    const instance = createFloatingElement(target, {
      content: createElementFromHTML('<div style="width: 200vw; height: 200vh"></div>'),
      trigger: 'manual',
      placement: 'bottom-end',
      maxWidth: 'none',
      limitSizeToViewport: {horizontal: true, vertical: true},
    });
    instance.show();
    await vi.waitFor(() => expect(instance.element.hasAttribute('data-placement')).toBe(true));
    const bounds = instance.element.getBoundingClientRect();
    expect(bounds.width).toBeGreaterThan(0);
    expect(bounds.height).toBeGreaterThan(0);
    expect(bounds.left).toBeGreaterThanOrEqual(0);
    expect(bounds.top).toBeGreaterThanOrEqual(0);
    expect(bounds.right).toBeLessThanOrEqual(window.innerWidth);
    expect(bounds.bottom).toBeLessThanOrEqual(window.innerHeight);
    const content = instance.element.querySelector('.floating-content')!;
    expect(content.scrollWidth).toBeGreaterThan(content.clientWidth);
    expect(content.scrollHeight).toBeGreaterThan(content.clientHeight);
    instance.destroy();
  });

  test('first hover preserves cursor position and accessible multiline tooltip text', async () => {
    const target = makeTarget();
    target.style.cssText = 'position: fixed; left: 50%; top: 50%';
    target.setAttribute('data-tooltip-content', '<text>\nsecond line');
    target.setAttribute('data-tooltip-follow-cursor', 'horizontal');
    target.setAttribute('aria-hidden', 'true');
    initGlobalTooltips();
    expect(target.getAttribute('aria-label')).toBe('<text>\nsecond line');
    expect(target.hasAttribute('aria-hidden')).toBe(false);
    const cursorX = target.getBoundingClientRect().left + 5;
    target.dispatchEvent(new MouseEvent('mouseover', {clientX: cursorX, bubbles: true}));
    const instance = getFloatingElement(target)!;
    expect(instance.props.placement).toBe('top');
    expect(instance.element.querySelector('.floating-content')!.innerHTML).toBe('&lt;text&gt;<br>second line');
    await vi.waitFor(() => expect(instance.element.hasAttribute('data-placement')).toBe(true));
    const bounds = instance.element.getBoundingClientRect();
    expect(Math.abs((bounds.left + bounds.right) / 2 - cursorX)).toBeLessThanOrEqual(1);
    instance.destroy();
  });
});
