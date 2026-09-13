import { fetchJSON } from "/api.js";
import { createCropEditor } from "/crop.js";
import { requireSource } from "/source-selection.js";
import {
  capitalize,
  formatBytes,
  imageMediaType,
} from "/app-utils.js";
import {
  createPreviewChangeHandler,
  createSizeComparison,
  exportRequest,
  setLiveStatus,
  setWorkflowStep,
  showImmediatePreviews,
  updateExportControls,
} from "/ui.js";
const elements = {
  source: document.querySelector("#source"),
  sourceSummary: document.querySelector("#source-summary"),
  sourceSize: document.querySelector("#source-size"),
  sourceDimensions: document.querySelector("#source-dimensions"),
  outputSize: document.querySelector("#output-size"),
  outputDimensions: document.querySelector("#output-dimensions"),
  sizeReduction: document.querySelector("#size-reduction"),
  outputSizeBar: document.querySelector("#output-size-bar"),
  sizeComparisonDetail: document.querySelector("#size-comparison-detail"),
  processorVersion: document.querySelector("#processor-version"),
  sourceSelection: document.querySelector("#source-selection"),
  sourceSelector: document.querySelector("#source-selector"),
  loadSource: document.querySelector("#load-source"),
  sourcePathSelection: document.querySelector("#source-path-selection"),
  sourceUpload: document.querySelector("#source-upload"),
  initialSourceFile: document.querySelector("#initial-source-file"),
  chooseSourceFile: document.querySelector("#choose-source-file"),
  processorControls: document.querySelector("#processor-controls"),
  workspace: document.querySelector("#workspace"),
  stage: document.querySelector("#stage"),
  cropBox: document.querySelector("#crop-box"),
  cropLabel: document.querySelector("#crop-label"),
  cropSummary: document.querySelector("#crop-summary"),
  resetCrops: document.querySelector("#reset-crops"),
  import: document.querySelector("#import"),
  sourceFile: document.querySelector("#source-file"),
  export: document.querySelector("#export"),
  format: document.querySelector("#format"),
  quality: document.querySelector("#quality"),
  qualityValue: document.querySelector("#quality-value"),
  lossless: document.querySelector("#lossless"),
  liveStatus: document.querySelector("#live-status"),
  workflowSteps: [...document.querySelectorAll("[data-workflow]")],
  tabs: [...document.querySelectorAll("[data-role]")],
  previewCards: Object.fromEntries(
    [...document.querySelectorAll("[data-preview-role]")].map((card) => [
      card.dataset.previewRole,
      card,
    ]),
  ),
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
  setLiveStatus(elements, "Source image imported. Both crop frames were reset for the new dimensions.");
} else if (new URLSearchParams(window.location.search).has("selected")) {
  setLiveStatus(elements, "Source changed. Both crop frames were reset for the selected source.");
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
const sizeComparison = createSizeComparison(elements, config.source.bytes);
elements.processorVersion.textContent = config.processor;
elements.sourceSummary.textContent = `${config.source.path} · ${config.source.width}×${config.source.height} · ${formatBytes(config.source.bytes)}`;
elements.sourceSize.textContent = formatBytes(config.source.bytes);
elements.sourceDimensions.textContent = `${config.source.width}×${config.source.height}`;
sizeComparison.reset();
setWorkflowStep(elements, "crop");
for (const output of config.outputs) {
  elements.previews[output.role].width = output.width;
  elements.previews[output.role].height = output.height;
}
updateExportControls(elements, config.outputs);

elements.source.src = config.source.url;
await elements.source.decode();
const editor = createCropEditor({
  stage: elements.stage,
  box: elements.cropBox,
  source: { width: config.source.width, height: config.source.height },
  specs: config.outputs,
  onChange: createPreviewChangeHandler(elements, specs, (crops) => (currentCrops = crops), scheduleEncodedPreview),
});

elements.resetCrops.addEventListener("click", () => {
  editor.reset();
  setLiveStatus(elements, "Both crop frames were centered on the source.");
});

for (const tab of elements.tabs) {
  tab.addEventListener("click", () => {
    const role = tab.dataset.role;
    editor.setActive(role);
    elements.cropLabel.textContent = capitalize(role);
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
  setLiveStatus(elements, "Validating and importing source image…");
  try {
    const mediaType = imageMediaType(file);
    if (!mediaType) {
      throw new Error("Choose a PNG, JPEG, or GIF image.");
    }
    await fetchJSON("/api/import", {
      method: "POST",
      headers: {
        "Content-Type": mediaType,
        "X-Unit-Art-Token": config.token,
      },
      body: file,
    });
    window.location.assign("/?imported=1");
  } catch (error) {
    setLiveStatus(elements, error.message);
    elements.import.disabled = false;
    elements.export.disabled = !hasEncodedPreview;
    elements.sourceFile.value = "";
  }
});

elements.export.addEventListener("click", async () => {
  elements.export.disabled = true;
  setLiveStatus(elements, "Writing the previewed outputs…");
  setWorkflowStep(elements, "export");
  try {
    const response = await fetchJSON("/api/export", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Unit-Art-Token": config.token,
      },
      body: JSON.stringify(exportRequest(elements, editor.getCrops())),
    });
    setLiveStatus(
      elements,
      response.outputs
        .map(
          (output) =>
            `${capitalize(output.role)}: ${output.width}×${output.height} ${output.format.toUpperCase()}, ${formatBytes(output.bytes)}\n${output.path}\nSHA-256 ${output.sha256}`,
        )
        .join("\n\n"),
    );
  } catch (error) {
    setLiveStatus(elements, error.message);
  } finally {
    elements.export.disabled = !hasEncodedPreview;
  }
});

function handleExportOptionChange() {
  updateExportControls(elements, config.outputs);
  showImmediatePreviews(elements, config.outputs);
  scheduleEncodedPreview();
}

function scheduleEncodedPreview() {
  if (!currentCrops) return;
  clearTimeout(previewTimer);
  previewController?.abort();
  hasEncodedPreview = false;
  elements.export.disabled = true;
  sizeComparison.reset("Rendering encoded sizes…");
  setLiveStatus(elements, "Rendering preview…");
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
      body: JSON.stringify(exportRequest(elements, currentCrops)),
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
        setLiveStatus(
          elements,
          `${output.format.toUpperCase()} preview cannot be decoded by this browser. The exact encoded size is still available.`,
        );
      }
      document.querySelector(`#${output.role}-size`).textContent =
        `${output.width}×${output.height} ${output.format.toUpperCase()} · ${formatBytes(output.bytes)}`;
    }
    sizeComparison.update(response.outputs);
    hasEncodedPreview = true;
    elements.export.disabled = false;
    setLiveStatus(elements, "Preview ready");
    setWorkflowStep(elements, "export");
  } catch (error) {
    if (error.name !== "AbortError" && sequence === previewSequence) {
      setLiveStatus(elements, `Preview failed: ${error.message}`);
    }
  }
}
