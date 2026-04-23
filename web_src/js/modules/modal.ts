type ModalOpts = {
  closable?: boolean;
  onApprove?: (this: HTMLElement) => boolean | void;
  onShow?: (this: HTMLElement) => void | Promise<void>;
  onHide?: (this: HTMLElement) => void;
  onHidden?: (this: HTMLElement) => void;
};

// thin wrapper around Fomantic's jQuery modal plugin so callers don't have to touch jQuery or fomanticQuery
export function showModal(el: Element | null, opts: ModalOpts = {}) {
  if (!el) return;
  const $el = $(el);
  if (Object.keys(opts).length) $el.modal(opts);
  $el.modal('show');
}

export function hideModal(el: Element | null) {
  if (!el) return;
  $(el).modal('hide');
}
