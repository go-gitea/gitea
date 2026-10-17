import {userEvent} from 'vitest/browser';
import {createFloatingElement, getFloatingElement, initGlobalTooltips, showTemporaryTooltip} from './floating.ts';
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

  test('destroy unregisters and removes a popup even when hiding is prevented', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'manual', onHide: () => false});
    expect(getFloatingElement(target)).toBe(instance);
    instance.show();
    instance.destroy();
    expect(getFloatingElement(target)).toBeNull();
    expect(instance.element.isConnected).toBe(false);
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

  test('hover delay is canceled by hiding or clicking before the popup appears', () => {
    vi.useFakeTimers({toFake: ['setTimeout', 'clearTimeout']});
    const target = makeTarget();
    const instance = createFloatingElement(target, {delay: [40, 0], hideOnClick: true});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    instance.hide();
    vi.advanceTimersByTime(40);
    expect(instance.state.isShown).toBe(false);
    target.dispatchEvent(new MouseEvent('mouseenter'));
    target.click();
    vi.advanceTimersByTime(40);
    expect(instance.state.isShown).toBe(false);
    target.dispatchEvent(new MouseEvent('mouseenter'));
    document.body.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
    vi.advanceTimersByTime(40);
    expect(instance.state.isShown).toBe(false);
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
    const instance = createFloatingElement(target, {trigger: 'click', interactive: true, hideOnClick: true});
    expect(target.getAttribute('aria-expanded')).toBe('false');
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    expect(target.getAttribute('aria-expanded')).toBe('true');
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(false);
    expect(target.getAttribute('aria-expanded')).toBe('false');
    instance.destroy();
  });

  test('focus click ignores only the click that follows focus', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'focus click'});
    target.dispatchEvent(new FocusEvent('focus'));
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('interactive focus can enter popup content', () => {
    vi.useFakeTimers({toFake: ['setTimeout', 'clearTimeout']});
    const target = makeTarget();
    const content = createElementFromHTML<HTMLInputElement>('<input>');
    const instance = createFloatingElement(target, {content, trigger: 'focus click', interactive: true});
    target.focus();
    content.focus();
    vi.advanceTimersByTime(1);
    expect(instance.state.isShown).toBe(true);
    expect(document.activeElement).toBe(content);
    instance.destroy();
  });

  test('interactive hover crosses the reference gap and closes outside the popup border', async () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {interactive: true, interactiveBorder: 5});
    vi.spyOn(target, 'getBoundingClientRect').mockReturnValue(new DOMRect(100, 150, 20, 20));
    vi.spyOn(instance.element, 'getBoundingClientRect').mockReturnValue(new DOMRect(100, 100, 100, 40));
    target.dispatchEvent(new MouseEvent('mouseenter', {clientX: 110, clientY: 155}));
    target.dispatchEvent(new MouseEvent('mouseleave', {clientX: 110, clientY: 149}));
    document.dispatchEvent(new PointerEvent('pointermove', {clientX: 110, clientY: 145}));
    await vi.waitFor(() => expect(instance.element.hasAttribute('data-placement')).toBe(true));
    expect(instance.state.isShown).toBe(true);
    instance.element.dispatchEvent(new MouseEvent('mouseenter'));
    expect(instance.state.isShown).toBe(true);
    instance.element.dispatchEvent(new MouseEvent('mouseleave', {clientX: 110, clientY: 100}));
    document.dispatchEvent(new PointerEvent('pointermove', {clientX: 300, clientY: 300}));
    await vi.waitFor(() => expect(instance.state.isShown).toBe(false));
    instance.destroy();
  });

  test('temporary feedback from popup content is dismissed by the next outside press', () => {
    const content = createElementFromHTML('<button>copy</button>');
    const popup = createFloatingElement(makeTarget(), {content, trigger: 'manual', interactive: true});
    popup.show();
    showTemporaryTooltip(content, 'Copied');
    const tooltip = getFloatingElement(document.body)!;
    expect(tooltip.state.isShown).toBe(true);
    document.body.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
    expect(tooltip.state.isShown).toBe(false);
    expect(getFloatingElement(document.body)).toBeNull();
    popup.destroy();
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
