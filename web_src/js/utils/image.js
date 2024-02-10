import pngChunksExtract from 'png-chunks-extract';

export async function imageInfo(file) {
  let width = 0;
  let dppx = 1;

  try {
    if (file.type === 'image/png') {
      const buffer = await file.arrayBuffer();
      const chunks = pngChunksExtract(new Uint8Array(buffer));

      // extract width from mandatory IHDR chunk
      const ihdr = chunks.find((chunk) => chunk.name === 'IHDR');
      if (ihdr?.data?.length) {
        const View = new DataView(ihdr.data.buffer, 0);
        width = View.getUint32(0);
      }

      // extract dppx from optional pHYs chunk, assuming unit is meter and pixels are square
      const phys = chunks.find((chunk) => chunk.name === 'pHYs');
      if (phys?.data?.length) {
        const view = new DataView(phys.data.buffer, 0);
        const dpi = Math.round(view.getUint32(0) / 39.3701);
        const unit = view.getUint8(8);
        if (unit !== 1) return 1;
        dppx = dpi / 72;
      } else {
        dppx = 1;
      }
    }
  } catch {}

  return {width, dppx};
}
