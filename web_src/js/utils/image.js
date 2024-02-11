export async function pngChunks(blob) {
  const uint8arr = new Uint8Array(await blob.arrayBuffer());
  const view = new DataView(uint8arr.buffer, 0);
  if (view.getBigUint64(0) !== 9894494448401390090n) throw new Error('Invalid png header');

  const decoder = new TextDecoder();
  const chunks = [];
  let index = 8;
  while (index < uint8arr.length) {
    const len = view.getUint32(index);
    chunks.push({
      name: decoder.decode(uint8arr.slice(index + 4, index + 8)),
      data: uint8arr.slice(index + 8, index + 8 + len),
    });
    index += len + 12;
  }

  return chunks;
}

export async function pngInfo(blob) {
  let width = 0;
  let dppx = 1;
  if (blob.type !== 'image/png') return {width, dppx};

  try {
    const chunks = await pngChunks(blob);

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
      if (unit !== 1) { // not meter
        dppx = 1;
      } else {
        dppx = Math.round(view.getUint32(0) / 39.3701) / 72; // meter to inch to dppx
      }
    } else {
      dppx = 1;
    }
  } catch {}

  return {width, dppx};
}
