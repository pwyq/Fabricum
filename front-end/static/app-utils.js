export function replaceExtension(path, format) {
  return `${path.slice(0, path.lastIndexOf("."))}.${format}`;
}

export function formatBytes(bytes) {
  return `${bytes.toLocaleString()} bytes`;
}

export function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

export function imageMediaType(file) {
  const type = file?.type?.toLowerCase();
  if (["image/png", "image/jpeg", "image/gif"].includes(type)) return type;
  const extension = file?.name?.slice(file.name.lastIndexOf(".")).toLowerCase();
  return {
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
  }[extension] ?? "";
}
