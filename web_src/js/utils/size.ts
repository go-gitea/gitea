const sizeUnits = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB'];

export function formatFileSize(size: number): string {
  if (!Number.isFinite(size) || size < 0) return '0 B';
  let value = size;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < sizeUnits.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  const formattedValue = unitIndex === 0 ? String(Math.round(value)) : value.toFixed(value >= 10 ? 0 : 1);
  return `${formattedValue} ${sizeUnits[unitIndex]}`;
}
