export function pngChunks(data) {
  const view = new DataView(data.buffer, 0);
  if (view.getBigUint64(0) !== 9894494448401390090n) throw new Error(`Invalid png header`);

  const decoder = new TextDecoder();
  const chunks = [];
  let index = 8;
  while (index < data.length) {
    const len = view.getUint32(index);
    chunks.push({
      name: decoder.decode(data.slice(index + 4, index + 8)),
      data: data.slice(index + 8, index + 8 + len),
    });
    index += len + 12;
  }

  return chunks;
}

export async function pngInfo(blob) {
  let width = 0;
  let dppx = 1;

  // only png is supported currently
  if (blob.type !== 'image/png') return {width, dppx};

  try {
    const chunks = pngChunks(new Uint8Array(await blob.arrayBuffer()));

    // extract width from mandatory IHDR chunk
    const ihdr = chunks.find((chunk) => chunk.name === 'IHDR');
    if (ihdr?.data?.length) {
      const view = new DataView(ihdr.data.buffer, 0);
      width = view.getUint32(0);
    }

    // extract dppx from optional pHYs chunk, assuming pixels are square
    const phys = chunks.find((chunk) => chunk.name === 'pHYs');
    if (phys?.data?.length) {
      const view = new DataView(phys.data.buffer, 0);
      const unit = view.getUint8(8);
      if (unit !== 1) return 1; // not meter
      const dpi = Math.round(view.getUint32(0) / 39.3701);
      dppx = dpi / 72;
    } else {
      dppx = 1;
    }
  } catch {}

  return {width, dppx};
}
