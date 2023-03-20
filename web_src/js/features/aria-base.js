let ariaIdCounter = 0;

export function generateAriaId() {
  return `_aria_auto_id_${ariaIdCounter++}`;
}
