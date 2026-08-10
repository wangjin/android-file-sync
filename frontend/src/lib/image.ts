import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

// Previewable image extensions. Keep in sync with the Go-side imageExts map in
// internal/adb/preview.go (IsImagePath).
const IMG_EXT = new Set(['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg'])

// isImageFile reports whether a file entry is a previewable image (not a dir,
// and with a recognized image extension). Used to decide whether double-click
// or the context menu offers "preview".
export function isImageFile(entry: FileEntry): boolean {
  if (entry.is_dir) return false
  const dot = entry.name.lastIndexOf('.')
  if (dot < 0 || dot === entry.name.length - 1) return false
  return IMG_EXT.has(entry.name.slice(dot + 1).toLowerCase())
}

// humanSize formats a byte count for display, e.g. "12.3 MB".
export function humanSize(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

// decodeBase64Bytes turns a Wails-transported []byte (which arrives as a
// base64 string over JSON RPC) into a Uint8Array for Blob construction.
// If the value is already binary (Uint8Array/ArrayBuffer), it passes through.
export function decodeBase64Bytes(v: unknown): Uint8Array {
  if (v instanceof Uint8Array) return v
  if (v instanceof ArrayBuffer) return new Uint8Array(v)
  if (typeof v === 'string') {
    const bin = atob(v)
    const out = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
    return out
  }
  // Fallback: treat as already-array-like.
  return new Uint8Array(v as any)
}
