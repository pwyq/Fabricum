import assert from "node:assert/strict";
import test from "node:test";
import { centeredCrop, moveCrop, resizeCrop } from "../static/crop.js";

const source = { width: 1376, height: 768 };
const squareSpec = { role: "square", width: 512, height: 512 };
const wideSpec = { role: "wide", width: 768, height: 576 };

test("centers the largest exact-aspect crop for each role", () => {
  assert.deepEqual(centeredCrop(source, squareSpec), {
    x: 304,
    y: 0,
    width: 768,
    height: 768,
  });
  assert.deepEqual(centeredCrop(source, wideSpec), {
    x: 176,
    y: 0,
    width: 1024,
    height: 768,
  });
  const squareSource = { width: 1024, height: 1024 };
  assert.deepEqual(centeredCrop(squareSource, squareSpec), {
    x: 0,
    y: 0,
    width: 1024,
    height: 1024,
  });
  assert.deepEqual(centeredCrop(squareSource, wideSpec), {
    x: 0,
    y: 128,
    width: 1024,
    height: 768,
  });
});

test("clamps crop movement to source bounds", () => {
  const square = centeredCrop(source, squareSpec);
  assert.deepEqual(moveCrop(square, -1000, 1000, source), {
    x: 0,
    y: 0,
    width: 768,
    height: 768,
  });
  assert.deepEqual(moveCrop(square, 1000, -1000, source), {
    x: 608,
    y: 0,
    width: 768,
    height: 768,
  });
});

test("resizes while preserving role ratio, bounds, and minimum size", () => {
  const wide = { x: 176, y: 96, width: 768, height: 576 };
  const enlarged = resizeCrop(wide, { x: 100, y: 20 }, "nw", wideSpec, source);
  assert.equal(
    enlarged.width * wideSpec.height,
    enlarged.height * wideSpec.width,
  );
  assert.ok(enlarged.x >= 0 && enlarged.y >= 0);
  assert.ok(enlarged.x + enlarged.width <= source.width);
  assert.ok(enlarged.y + enlarged.height <= source.height);

  const minimum = resizeCrop(wide, { x: 940, y: 668 }, "se", wideSpec, source);
  assert.deepEqual(minimum, wide);
});
