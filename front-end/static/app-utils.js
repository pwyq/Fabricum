export function replaceExtension(path, format) {
  return `${path.slice(0, path.lastIndexOf("."))}.${format}`;
}

export function formatBytes(bytes) {
  if (!Number.isFinite(bytes)) return "—";
  if (bytes < 1024) return `${Math.round(bytes).toLocaleString()} bytes`;
  const units = ["KB", "MB", "GB"];
  let value = bytes;
  let unit = "bytes";
  for (const candidate of units) {
    value /= 1024;
    unit = candidate;
    if (value < 1024 || candidate === units[units.length - 1]) break;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${unit}`;
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
