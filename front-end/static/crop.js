export function createCropEditor({ stage, box, source, specs, onChange }) {
  const initialCrops = Object.fromEntries(
    specs.map((spec) => [spec.role, centeredCrop(source, spec)]),
  );
  const crops = structuredClone(initialCrops);
  let activeRole = specs[0].role;
  let pointerAction = null;

  function setActive(role) {
    activeRole = role;
    box.className = `crop-box crop-box--${role}`;
    box.setAttribute("aria-label", `${capitalize(role)} crop area`);
    render();
  }

  function render() {
    const crop = crops[activeRole];
    box.style.left = `${(crop.x / source.width) * 100}%`;
    box.style.top = `${(crop.y / source.height) * 100}%`;
    box.style.width = `${(crop.width / source.width) * 100}%`;
    box.style.height = `${(crop.height / source.height) * 100}%`;
    onChange(activeRole, structuredClone(crops));
  }

  function pointFromEvent(event) {
    const bounds = stage.getBoundingClientRect();
    return {
      x: ((event.clientX - bounds.left) / bounds.width) * source.width,
      y: ((event.clientY - bounds.top) / bounds.height) * source.height,
    };
  }

  box.addEventListener("pointerdown", (event) => {
    const point = pointFromEvent(event);
    pointerAction = {
      handle: event.target.dataset.handle ?? "move",
      start: point,
      crop: structuredClone(crops[activeRole]),
    };
    box.setPointerCapture(event.pointerId);
    event.preventDefault();
  });

  box.addEventListener("pointermove", (event) => {
    if (!pointerAction || !box.hasPointerCapture(event.pointerId)) return;
    const point = pointFromEvent(event);
    crops[activeRole] =
      pointerAction.handle === "move"
        ? moveCrop(
            pointerAction.crop,
            point.x - pointerAction.start.x,
            point.y - pointerAction.start.y,
            source,
          )
        : resizeCrop(
            pointerAction.crop,
            point,
            pointerAction.handle,
            specFor(activeRole),
            source,
          );
    render();
  });

  box.addEventListener("pointerup", (event) => {
    if (box.hasPointerCapture(event.pointerId))
      box.releasePointerCapture(event.pointerId);
    pointerAction = null;
  });

  box.addEventListener("pointercancel", () => {
    pointerAction = null;
  });

  box.addEventListener("keydown", (event) => {
    const offsets = {
      ArrowLeft: [-1, 0],
      ArrowRight: [1, 0],
      ArrowUp: [0, -1],
      ArrowDown: [0, 1],
    };
    const offset = offsets[event.key];
    if (!offset) return;
    const amount = event.shiftKey ? 10 : 1;
    crops[activeRole] = moveCrop(
      crops[activeRole],
      offset[0] * amount,
      offset[1] * amount,
      source,
    );
    render();
    event.preventDefault();
  });

  function specFor(role) {
    return specs.find((spec) => spec.role === role);
  }

  render();
  return {
    setActive,
    getCrops: () => structuredClone(crops),
    reset: () => {
      crops[activeRole] = structuredClone(initialCrops[activeRole]);
      render();
    },
  };
}

export function centeredCrop(source, spec) {
  const divisor = greatestCommonDivisor(spec.width, spec.height);
  const widthStep = spec.width / divisor;
  const heightStep = spec.height / divisor;
  const scale = Math.floor(
    Math.min(source.width / widthStep, source.height / heightStep),
  );
  const width = widthStep * scale;
  const height = heightStep * scale;
  return {
    x: Math.floor((source.width - width) / 2),
    y: Math.floor((source.height - height) / 2),
    width,
    height,
  };
}

export function moveCrop(crop, deltaX, deltaY, source) {
  return {
    ...crop,
    x: clamp(Math.round(crop.x + deltaX), 0, source.width - crop.width),
    y: clamp(Math.round(crop.y + deltaY), 0, source.height - crop.height),
  };
}

export function resizeCrop(crop, point, handle, spec, source) {
  const fromWest = handle.includes("w");
  const fromNorth = handle.includes("n");
  const anchorX = fromWest ? crop.x + crop.width : crop.x;
  const anchorY = fromNorth ? crop.y + crop.height : crop.y;
  const horizontalRoom = fromWest ? anchorX : source.width - anchorX;
  const verticalRoom = fromNorth ? anchorY : source.height - anchorY;
  const ratio = spec.width / spec.height;
  const desiredWidth = Math.max(
    Math.abs(point.x - anchorX),
    Math.abs(point.y - anchorY) * ratio,
  );
  const maximumWidth = Math.min(horizontalRoom, verticalRoom * ratio);
  const step = spec.width / greatestCommonDivisor(spec.width, spec.height);
  const width = clamp(
    Math.floor(desiredWidth / step) * step,
    spec.width,
    Math.floor(maximumWidth / step) * step,
  );
  const height = (width * spec.height) / spec.width;
  return {
    x: fromWest ? anchorX - width : anchorX,
    y: fromNorth ? anchorY - height : anchorY,
    width,
    height,
  };
}

function greatestCommonDivisor(left, right) {
  while (right !== 0) [left, right] = [right, left % right];
  return left;
}

function clamp(value, minimum, maximum) {
  return Math.min(Math.max(value, minimum), maximum);
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
