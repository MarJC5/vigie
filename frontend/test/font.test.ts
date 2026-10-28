// Version columns use tabular figures: the bundled typeface must provide the OpenType `tnum`
// feature. WOFF2 stores the font tables brotli-compressed after its table directory; the feature
// tag appears in the GSUB table once decompressed.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { brotliDecompressSync } from "node:zlib";
import { describe, expect, it } from "vitest";

const require = createRequire(import.meta.url);
// Digits (U+0030 to U+0039) are in the "latin" subset only (Fontsource unicode.json: latin covers
// U+0000-00FF); the "latin-ext" file has no digits, so no tabular figures to carry.
const files = [require.resolve("@fontsource-variable/instrument-sans/files/instrument-sans-latin-wght-normal.woff2")];

/** Decompressed font tables of a WOFF2 file (header 48 bytes, then a variable-length directory). */
function tables(path: string): Buffer {
  const data = readFileSync(path);
  expect(data.toString("latin1", 0, 4)).toBe("wOF2");
  const numTables = data.readUInt16BE(12);
  const compressedLength = data.readUInt32BE(20);
  let offset = 48;
  const base128 = () => {
    let value = 0;
    for (let i = 0; i < 5; i++) {
      const byte = data[offset++]!;
      value = value * 128 + (byte & 0x7f);
      if ((byte & 0x80) === 0) return value;
    }
    throw new Error("bad UIntBase128");
  };
  for (let i = 0; i < numTables; i++) {
    const flags = data[offset++]!;
    if ((flags & 0x3f) === 0x3f) offset += 4; // arbitrary tag
    const tag = flags & 0x3f;
    const transform = (flags >> 6) & 0x03;
    base128(); // origLength
    const transformed = tag === 10 || tag === 11 ? transform === 0 : transform !== 0; // glyf, loca
    if (transformed) base128(); // transformLength
  }
  return brotliDecompressSync(data.subarray(offset, offset + compressedLength));
}

describe("Instrument Sans", () => {
  it.each(files)("has tabular figures (tnum) in %s", (path) => {
    expect(tables(path).includes(Buffer.from("tnum", "latin1"))).toBe(true);
  });
});

describe("--vg-font", () => {
  it("starts with the family the bundled font declares", () => {
    const fontCss = readFileSync(require.resolve("@fontsource-variable/instrument-sans/wght.css"), "utf8");
    const declared = /font-family:\s*'([^']+)'/.exec(fontCss)?.[1];
    const tokens = readFileSync(new URL("../src/styles/tokens.css", import.meta.url), "utf8");
    const first = /--vg-font:\s*"([^"]+)"/.exec(tokens)?.[1];
    expect(declared).toBeDefined();
    expect(first).toBe(declared);
  });
});
