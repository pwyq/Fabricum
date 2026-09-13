import { imageMediaType } from "./app-utils.js";
import { setLiveStatus } from "./ui.js";

export async function requireSource(
  elements,
  initialConfig,
  fetchJSON,
  onSourceChanged,
) {
  const sources = initialConfig.sources ?? [];
  const hasSource = Boolean(initialConfig.source);
  const hasSourceList = sources.length > 0;
  if (hasSource && !hasSourceList) return initialConfig;

  let resolveSource;
  const sourceConfig = hasSource
    ? Promise.resolve(initialConfig)
    : new Promise((resolve) => {
        resolveSource = resolve;
      });

  for (const sourcePath of sources) {
    const option =
      elements.sourceSelector.ownerDocument.createElement("option");
    option.value = sourcePath;
    option.textContent = sourcePath;
    elements.sourceSelector.append(option);
  }
  if (hasSource) elements.sourceSelector.value = initialConfig.source.path;
  elements.sourceSelection.hidden = false;
  elements.sourcePathSelection.hidden = !hasSourceList;
  elements.sourceUpload.hidden = hasSourceList;

  if (!hasSource) {
    elements.sourceSummary.textContent = "Choose a source image.";
    elements.processorControls.hidden = true;
    elements.workspace.hidden = true;
    setLiveStatus(elements, "Choose a source image to begin.");
  }

  const showEditor = () => {
    elements.sourceSelection.hidden = true;
    elements.processorControls.hidden = false;
    elements.workspace.hidden = false;
    setLiveStatus(elements, "Adjust both crops, then export.");
  };

  if (hasSourceList) {
    elements.loadSource.textContent = hasSource ? "Change source" : "Load source";
    elements.loadSource.addEventListener("click", async () => {
      const sourcePath = elements.sourceSelector.value;
      if (!sourcePath) {
        setLiveStatus(elements, "Choose a source image.");
        return;
      }
      elements.loadSource.disabled = true;
      setLiveStatus(elements, "Validating source…");
      try {
        const selectedConfig = await fetchJSON("/api/source", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Unit-Art-Token": initialConfig.token,
          },
          body: JSON.stringify({ path: sourcePath }),
        });
        if (hasSource) {
          onSourceChanged();
          return;
        }
        showEditor();
        resolveSource(selectedConfig);
      } catch (error) {
        setLiveStatus(elements, error.message);
        elements.loadSource.disabled = false;
      }
    });
  }

  if (!hasSource && !hasSourceList) {
    elements.chooseSourceFile.addEventListener("click", () =>
      elements.initialSourceFile.click(),
    );
    elements.initialSourceFile.addEventListener("change", async () => {
      const [file] = elements.initialSourceFile.files;
      if (!file) return;
      const mediaType = imageMediaType(file);
      if (!mediaType) {
        setLiveStatus(elements, "Choose a PNG, JPEG, or GIF image.");
        elements.initialSourceFile.value = "";
        return;
      }
      elements.chooseSourceFile.disabled = true;
      setLiveStatus(elements, "Validating source image…");
      try {
        const selectedConfig = await fetchJSON("/api/source-upload", {
          method: "POST",
          headers: {
            "Content-Type": mediaType,
            "X-Unit-Art-Token": initialConfig.token,
            "X-Fabricum-Source-Name": file.name,
          },
          body: file,
        });
        showEditor();
        resolveSource(selectedConfig);
      } catch (error) {
        setLiveStatus(elements, error.message);
        elements.chooseSourceFile.disabled = false;
        elements.initialSourceFile.value = "";
      }
    });
  }

  return sourceConfig;
}
