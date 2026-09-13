import assert from "node:assert/strict";
import test from "node:test";
import { createPreviewChangeHandler, exportRequest } from "../static/ui.js";

function previewCanvas() {
  const canvas = {
    width: 8,
    height: 8,
    hidden: true,
    drawCount: 0,
    getContext: () => ({
      clearRect() {},
      drawImage() {
        canvas.drawCount += 1;
      },
    }),
  };
  return canvas;
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
  assert.equal(elements.previews.square.drawCount, 1);
  assert.equal(elements.previews.wide.drawCount, 0);

  handleChange("wide", crops);
  assert.equal(elements.previewCards.square.hidden, true);
  assert.equal(elements.previewCards.wide.hidden, false);
  assert.equal(elements.previews.square.drawCount, 1);
  assert.equal(elements.previews.wide.drawCount, 1);
});

test("requests only the selected output role", () => {
  const request = exportRequest(
    {
      cropRole: { value: "wide" },
      format: { value: "webp" },
      quality: { value: "84" },
      lossless: { checked: false },
    },
    {
      square: { x: 1, y: 1, width: 6, height: 6 },
      wide: { x: 0, y: 1, width: 8, height: 6 },
    },
  );

  assert.equal(request.role, "wide");
  assert.equal(request.format, "webp");
  assert.equal(request.quality, 84);
});
