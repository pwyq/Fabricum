import { capitalize, formatBytes, replaceExtension } from "./app-utils.js";

export function createSizeComparison(elements, sourceBytes) {
  return {
    reset(message = "Generate a preview to compare encoded output size.") {
      elements.outputSize.textContent = "—";
      elements.outputDimensions.textContent = "Preview required";
      elements.sizeReduction.textContent = message;
      elements.sizeReduction.className = "size-reduction size-reduction--pending";
      elements.outputSizeBar.style.width = "0%";
      elements.sizeComparisonDetail.textContent = message;
    },
    update(outputs) {
      const outputBytes = outputs.reduce((total, output) => total + output.bytes, 0);
      const change =
        sourceBytes > 0 ? ((sourceBytes - outputBytes) / sourceBytes) * 100 : 0;
      const absoluteChange = Math.abs(change);
      const changeLabel = `${absoluteChange.toFixed(absoluteChange >= 10 ? 0 : 1)}% ${change >= 0 ? "smaller" : "larger"}`;
      elements.outputSize.textContent = formatBytes(outputBytes);
      elements.outputDimensions.textContent = outputs
        .map((output) => `${output.width}×${output.height}`)
        .join(" + ");
      elements.sizeReduction.textContent = changeLabel;
      elements.sizeReduction.className = `size-reduction${change < 0 ? " size-reduction--larger" : ""}`;
      elements.outputSizeBar.style.width = `${Math.min(100, Math.max(0, sourceBytes > 0 ? (outputBytes / sourceBytes) * 100 : 0))}%`;
      elements.sizeComparisonDetail.textContent = `${formatBytes(sourceBytes)} source → ${formatBytes(outputBytes)} across ${outputs.length} outputs`;
    },
  };
}

export function createPreviewChangeHandler(
  elements,
  specs,
  setCrops,
  schedulePreview,
) {
  return (activeRole, crops) => {
    setCrops(crops);
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
    setActivityStatus(elements, "Preview updating…");
    schedulePreview();
  };
}

export function setActivityStatus(elements, message, kind = "") {
  elements.previewStatus.textContent = message;
  elements.previewStatus.className = `activity-status${kind ? ` activity-status--${kind}` : ""}`;
}

export function setWorkflowStep(elements, activeStep) {
  const activeIndex = elements.workflowSteps.findIndex(
    (step) => step.dataset.workflow === activeStep,
  );
  for (const [index, step] of elements.workflowSteps.entries()) {
    step.classList.toggle("workflow-step--active", index === activeIndex);
    step.classList.toggle("workflow-step--complete", index < activeIndex);
  }
}

export function updateExportControls(elements, outputs) {
  const format = elements.format.value;
  const hasQuality = format !== "png" && !elements.lossless.checked;
  elements.quality.disabled = !hasQuality;
  elements.lossless.disabled = format === "png";
  elements.qualityValue.textContent = hasQuality ? elements.quality.value : "—";
  for (const output of outputs) {
    document.querySelector(`#${output.role}-size`).textContent =
      `${output.width}×${output.height} ${format.toUpperCase()} · estimating…`;
    document.querySelector(`#${output.role}-path`).textContent = replaceExtension(
      output.path,
      format,
    );
  }
}

export function showImmediatePreviews(elements, outputs) {
  for (const output of outputs) {
    elements.previews[output.role].hidden = false;
    elements.encodedPreviews[output.role].hidden = true;
  }
}

export function exportRequest(elements, crops) {
  const format = elements.format.value;
  return {
    ...crops,
    format,
    quality: format === "png" ? 0 : Number(elements.quality.value),
    lossless: format === "png" ? false : elements.lossless.checked,
  };
}
