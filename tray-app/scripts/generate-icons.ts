// This script generates placeholder tray icons as 22x22 colored PNGs.
// Run with: npx ts-node scripts/generate-icons.ts
// For production, replace with proper designed icons.

import * as fs from "fs";
import * as path from "path";

// Minimal 22x22 PNG with a single solid color
// PNG structure: signature + IHDR + IDAT (uncompressed) + IEND
function createSolidPNG(r: number, g: number, b: number): Buffer {
  // For development, create a 1x1 PNG that Electron will scale
  const width = 16;
  const height = 16;

  // This is a placeholder. In production, use proper icon design tools.
  // For now, write empty files that Electron can load without crashing.
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

  // IHDR chunk
  const ihdrData = Buffer.alloc(13);
  ihdrData.writeUInt32BE(width, 0);
  ihdrData.writeUInt32BE(height, 4);
  ihdrData[8] = 8; // bit depth
  ihdrData[9] = 2; // color type (RGB)
  ihdrData[10] = 0; // compression
  ihdrData[11] = 0; // filter
  ihdrData[12] = 0; // interlace

  const ihdr = createChunk("IHDR", ihdrData);

  // IDAT chunk - raw pixel data with zlib wrapper
  const rawData = Buffer.alloc(height * (1 + width * 3));
  for (let y = 0; y < height; y++) {
    const offset = y * (1 + width * 3);
    rawData[offset] = 0; // filter: none
    for (let x = 0; x < width; x++) {
      const pixelOffset = offset + 1 + x * 3;
      rawData[pixelOffset] = r;
      rawData[pixelOffset + 1] = g;
      rawData[pixelOffset + 2] = b;
    }
  }

  // Minimal zlib: deflate with no compression
  const zlibHeader = Buffer.from([0x78, 0x01]);
  // Split into blocks of max 65535 bytes
  const blocks: Buffer[] = [];
  let remaining = rawData.length;
  let pos = 0;
  while (remaining > 0) {
    const blockSize = Math.min(remaining, 65535);
    const isFinal = remaining <= 65535;
    const blockHeader = Buffer.alloc(5);
    blockHeader[0] = isFinal ? 0x01 : 0x00;
    blockHeader.writeUInt16LE(blockSize, 1);
    blockHeader.writeUInt16LE(blockSize ^ 0xffff, 3);
    blocks.push(blockHeader);
    blocks.push(rawData.subarray(pos, pos + blockSize));
    pos += blockSize;
    remaining -= blockSize;
  }

  // Adler-32 checksum
  let s1 = 1;
  let s2 = 0;
  for (let i = 0; i < rawData.length; i++) {
    s1 = (s1 + rawData[i]) % 65521;
    s2 = (s2 + s1) % 65521;
  }
  const adler32 = Buffer.alloc(4);
  const adler32Value = ((s2 * 65536 + s1) >>> 0);
  adler32.writeUInt32BE(adler32Value, 0);

  const zlibData = Buffer.concat([zlibHeader, ...blocks, adler32]);
  const idat = createChunk("IDAT", zlibData);

  // IEND chunk
  const iend = createChunk("IEND", Buffer.alloc(0));

  return Buffer.concat([signature, ihdr, idat, iend]);
}

function createChunk(type: string, data: Buffer): Buffer {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length, 0);
  const typeBuffer = Buffer.from(type, "ascii");
  const crcInput = Buffer.concat([typeBuffer, data]);

  // CRC32
  let crc = 0xffffffff;
  for (let i = 0; i < crcInput.length; i++) {
    crc ^= crcInput[i];
    for (let j = 0; j < 8; j++) {
      crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0);
    }
  }
  crc ^= 0xffffffff;
  const crcBuffer = Buffer.alloc(4);
  crcBuffer.writeUInt32BE(crc >>> 0, 0);

  return Buffer.concat([length, typeBuffer, data, crcBuffer]);
}

const icons: Array<{ name: string; r: number; g: number; b: number }> = [
  { name: "tray-idle", r: 128, g: 128, b: 128 },
  { name: "tray-joining", r: 230, g: 190, b: 30 },
  { name: "tray-recording", r: 220, g: 50, b: 50 },
  { name: "tray-processing", r: 50, g: 120, b: 220 },
  { name: "tray-error", r: 200, g: 30, b: 30 },
];

const assetsDir = path.join(__dirname, "..", "assets");
fs.mkdirSync(assetsDir, { recursive: true });

for (const icon of icons) {
  const png = createSolidPNG(icon.r, icon.g, icon.b);
  fs.writeFileSync(path.join(assetsDir, `${icon.name}.png`), png);
  // 2x version for retina
  fs.writeFileSync(path.join(assetsDir, `${icon.name}@2x.png`), png);
  console.log(`Generated ${icon.name}.png`);
}
