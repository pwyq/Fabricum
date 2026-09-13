import assert from "node:assert/strict";
import test from "node:test";
import { createPreviewChangeHandler } from "../static/ui.js";

function previewCanvas() {
  return {
    width: 8,
    height: 8,
    hidden: true,
    getContext: () => ({ clearRect() {}, drawImage() {} }),
  };
}

test("shows only the preview for the active crop role", () => {
  const elements = {
    cropSummary: { textContent: "" },
    source: {},
    liveStatus: { textContent: "" },
    previewCards: {
      square: { hidden: false },
      wide: { hidden: true },
    },
    previews: {
      square: previewCanvas(),
      wide: previewCanvas(),
    },
    encodedPreviews: {
      square: { hidden: false },
      wide: { hidden: false },
    },
  };
  const specs = {
    square: { width: 8, height: 8 },
    wide: { width: 8, height: 6 },
  };
  const crops = {
    square: { x: 0, y: 0, width: 8, height: 8 },
    wide: { x: 0, y: 1, width: 8, height: 6 },
  };
  const handleChange = createPreviewChangeHandler(
    elements,
    specs,
    () => {},
    () => {},
  );

  handleChange("square", crops);
  assert.equal(elements.previewCards.square.hidden, false);
  assert.equal(elements.previewCards.wide.hidden, true);

  handleChange("wide", crops);
  assert.equal(elements.previewCards.square.hidden, true);
  assert.equal(elements.previewCards.wide.hidden, false);
});
