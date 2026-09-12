export function replaceExtension(path, format) {
  return `${path.slice(0, path.lastIndexOf("."))}.${format}`;
}

export function formatBytes(bytes) {
  return `${bytes.toLocaleString()} bytes`;
}

export function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
