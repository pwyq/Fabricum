export async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  const body = await response.json();
  if (!response.ok)
    throw new Error(body.error ?? `${response.status} ${response.statusText}`);
  return body;
}
