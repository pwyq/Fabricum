export async function requireSource(
  elements,
  initialConfig,
  fetchJSON,
  onSourceChanged,
) {
  const hasSource = Boolean(initialConfig.source);
  if (hasSource && initialConfig.sources.length === 0) return initialConfig;
  let resolveSource;
  const sourceConfig = hasSource
    ? Promise.resolve(initialConfig)
    : new Promise((resolve) => {
        resolveSource = resolve;
      });

  for (const sourcePath of initialConfig.sources) {
    const option =
      elements.sourceSelector.ownerDocument.createElement("option");
    option.value = sourcePath;
    option.textContent = sourcePath;
    elements.sourceSelector.append(option);
  }
  if (hasSource) elements.sourceSelector.value = initialConfig.source.path;
  elements.loadSource.textContent = hasSource ? "Change source" : "Load source";
  elements.sourceSelection.hidden = false;
  if (!hasSource) {
    elements.sourceSummary.textContent = "Choose a source image.";
    elements.processorControls.hidden = true;
    elements.workspace.hidden = true;
    elements.result.textContent =
      "Choose an source image to begin.";
  }

  elements.loadSource.addEventListener("click", async () => {
    const sourcePath = elements.sourceSelector.value;
    if (!sourcePath) {
      elements.result.textContent = "Choose a source image.";
      return;
    }
    elements.loadSource.disabled = true;
    elements.result.textContent = "Validating source…";
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
      elements.sourceSelection.hidden = true;
      elements.processorControls.hidden = false;
      elements.workspace.hidden = false;
      elements.result.textContent = "Adjust both crops, then export.";
      resolveSource(selectedConfig);
    } catch (error) {
      elements.result.textContent = error.message;
      elements.loadSource.disabled = false;
    }
  });

  return sourceConfig;
}
