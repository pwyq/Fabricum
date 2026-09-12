import { fetchJSON } from "/api.js";
import { createCropEditor } from "/crop.js";
import { requireSource } from "/source-selection.js";
import { capitalize, formatBytes, replaceExtension } from "/app-utils.js";
const elements = {
  source: document.querySelector("#source"),
  sourceSummary: document.querySelector("#source-summary"),
  sourceSelection: document.querySelector("#source-selection"),
  sourceSelector: document.querySelector("#source-selector"),
  loadSource: document.querySelector("#load-source"),
  processorControls: document.querySelector("#processor-controls"),
  workspace: document.querySelector("#workspace"),
  stage: document.querySelector("#stage"),
  cropBox: document.querySelector("#crop-box"),
  cropLabel: document.querySelector("#crop-label"),
  cropSummary: document.querySelector("#crop-summary"),
  import: document.querySelector("#import"),
  sourceFile: document.querySelector("#source-file"),
  export: document.querySelector("#export"),
  format: document.querySelector("#format"),
  quality: document.querySelector("#quality"),
  qualityValue: document.querySelector("#quality-value"),
  lossless: document.querySelector("#lossless"),
  result: document.querySelector("#result"),
  tabs: [...document.querySelectorAll("[data-role]")],
  previews: {
    square: document.querySelector("#square-preview"),
    wide: document.querySelector("#wide-preview"),
  },
  encodedPreviews: {
    square: document.querySelector("#square-encoded-preview"),
    wide: document.querySelector("#wide-encoded-preview"),
  },
};
let currentCrops;
let previewTimer;
let previewController;
let previewSequence = 0;
let hasEncodedPreview = false;
if (new URLSearchParams(window.location.search).has("imported")) {
  elements.result.textContent =
    "Source PNG imported. Both crop frames were reset for the new dimensions.";
} else if (new URLSearchParams(window.location.search).has("selected")) {
  elements.result.textContent =
    "Source changed. Both crop frames were reset for the selected source.";
}

const config = await requireSource(
  elements,
  await fetchJSON("/api/config"),
  fetchJSON,
  () => window.location.assign("/?selected=1"),
);
const specs = Object.fromEntries(
  config.outputs.map((output) => [output.role, output]),
);
elements.sourceSummary.textContent = `${config.source.path} · ${config.source.width}×${config.source.height} · ${config.processor}`;
for (const output of config.outputs) {
  elements.previews[output.role].width = output.width;
  elements.previews[output.role].height = output.height;
}
updateExportControls();

elements.source.src = config.source.url;
await elements.source.decode();
const editor = createCropEditor({
  stage: elements.stage,
  box: elements.cropBox,
  source: { width: config.source.width, height: config.source.height },
  specs: config.outputs,
  onChange: updatePreviews,
});

for (const tab of elements.tabs) {
  tab.addEventListener("click", () => {
    const role = tab.dataset.role;
    editor.setActive(role);
    elements.cropLabel.textContent = role;
    for (const candidate of elements.tabs)
      candidate.setAttribute("aria-pressed", String(candidate === tab));
  });
}

elements.import.addEventListener("click", () => elements.sourceFile.click());
elements.format.addEventListener("change", handleExportOptionChange);
elements.quality.addEventListener("input", handleExportOptionChange);
elements.lossless.addEventListener("change", handleExportOptionChange);
elements.sourceFile.addEventListener("change", async () => {
  const [file] = elements.sourceFile.files;
  if (!file) return;
  if (!window.confirm(`Replace ${config.source.path} with ${file.name}?`)) {
    elements.sourceFile.value = "";
    return;
  }
  elements.import.disabled = true;
  elements.export.disabled = true;
  elements.result.textContent = "Validating and importing source PNG…";
  try {
    await fetchJSON("/api/import", {
      method: "POST",
      headers: {
        "Content-Type": "image/png",
        "X-Unit-Art-Token": config.token,
      },
      body: file,
    });
    window.location.assign("/?imported=1");
  } catch (error) {
    elements.result.textContent = error.message;
    elements.import.disabled = false;
    elements.export.disabled = !hasEncodedPreview;
    elements.sourceFile.value = "";
  }
});

elements.export.addEventListener("click", async () => {
  elements.export.disabled = true;
  elements.result.textContent = "Writing the previewed outputs…";
  try {
    const response = await fetchJSON("/api/export", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Unit-Art-Token": config.token,
      },
      body: JSON.stringify(exportRequest(editor.getCrops())),
    });
    elements.result.textContent = response.outputs
      .map(
        (output) =>
          `${capitalize(output.role)}: ${output.width}×${output.height} ${output.format.toUpperCase()}, ${formatBytes(output.bytes)}\n${output.path}\nSHA-256 ${output.sha256}`,
      )
      .join("\n\n");
  } catch (error) {
    elements.result.textContent = error.message;
  } finally {
    elements.export.disabled = !hasEncodedPreview;
  }
});

function updatePreviews(activeRole, crops) {
  currentCrops = crops;
  const active = crops[activeRole];
  elements.cropSummary.textContent = `${capitalize(activeRole)} crop · x ${active.x}, y ${active.y}, ${active.width}×${active.height}`;
  for (const [role, crop] of Object.entries(crops)) {
    const canvas = elements.previews[role];
    const context = canvas.getContext("2d");
    context.clearRect(0, 0, canvas.width, canvas.height);
    context.drawImage(
      elements.source,
      crop.x,
      crop.y,
      crop.width,
      crop.height,
      0,
      0,
      specs[role].width,
      specs[role].height,
    );
    canvas.hidden = false;
    elements.encodedPreviews[role].hidden = true;
  }
  scheduleEncodedPreview();
}

function handleExportOptionChange() {
  updateExportControls();
  showImmediatePreviews();
  scheduleEncodedPreview();
}

function updateExportControls() {
  const format = elements.format.value;
  const hasQuality = format !== "png" && !elements.lossless.checked;
  elements.quality.disabled = !hasQuality;
  elements.lossless.disabled = format === "png";
  elements.qualityValue.textContent = hasQuality ? elements.quality.value : "—";
  for (const output of config.outputs) {
    document.querySelector(`#${output.role}-size`).textContent =
      `${output.width}×${output.height} ${format.toUpperCase()} · estimating…`;
    document.querySelector(`#${output.role}-path`).textContent = replaceExtension(output.path, format);
  }
}

function showImmediatePreviews() {
  for (const output of config.outputs) {
    elements.previews[output.role].hidden = false;
    elements.encodedPreviews[output.role].hidden = true;
  }
}

function scheduleEncodedPreview() {
  if (!currentCrops) return;
  clearTimeout(previewTimer);
  previewController?.abort();
  hasEncodedPreview = false;
  elements.export.disabled = true;
  for (const output of config.outputs) {
    document.querySelector(`#${output.role}-size`).textContent =
      `${output.width}×${output.height} ${elements.format.value.toUpperCase()} · estimating…`;
  }
  const sequence = ++previewSequence;
  previewTimer = setTimeout(() => loadEncodedPreview(sequence), 450);
}

async function loadEncodedPreview(sequence) {
  previewController = new AbortController();
  try {
    const response = await fetchJSON("/api/preview", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Unit-Art-Token": config.token,
      },
      body: JSON.stringify(exportRequest(currentCrops)),
      signal: previewController.signal,
    });
    if (sequence !== previewSequence) return;
    for (const output of response.outputs) {
      const image = elements.encodedPreviews[output.role];
      image.src = response.dataUrls[output.role];
      try {
        await image.decode();
        if (sequence !== previewSequence) return;
        image.hidden = false;
        elements.previews[output.role].hidden = true;
      } catch {
        elements.result.textContent = `${output.format.toUpperCase()} preview cannot be decoded by this browser. The exact encoded size is still available.`;
      }
      document.querySelector(`#${output.role}-size`).textContent =
        `${output.width}×${output.height} ${output.format.toUpperCase()} · ${formatBytes(output.bytes)}`;
    }
    hasEncodedPreview = true;
    elements.export.disabled = false;
  } catch (error) {
    if (error.name !== "AbortError" && sequence === previewSequence) {
      elements.result.textContent = `Preview failed: ${error.message}`;
    }
  }
}

function exportRequest(crops) {
  const format = elements.format.value;
  return {
    ...crops,
    format,
    quality: format === "png" ? 0 : Number(elements.quality.value),
    lossless: format === "png" ? false : elements.lossless.checked,
  };
}
