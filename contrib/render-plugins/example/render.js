const TEXT_COLOR = '#f6e05e';
const BACKGROUND_COLOR = '#1a202c';

async function render(container, fileUrl) {
  container.innerHTML = '';

  const message = document.createElement('div');
  message.className = 'ui tiny message';
  message.textContent = 'Rendered by example-highlight-txt plugin';
  container.append(message);

  const response = await fetch(fileUrl);
  if (!response.ok) {
    throw new Error(`Failed to download file (${response.status})`);
  }
  const text = await response.text();

  const pre = document.createElement('pre');
  pre.style.backgroundColor = BACKGROUND_COLOR;
  pre.style.color = TEXT_COLOR;
  pre.style.padding = '1rem';
  pre.style.borderRadius = '0.5rem';
  pre.style.overflow = 'auto';
  pre.textContent = text;
  container.append(pre);
}

export default {render};
