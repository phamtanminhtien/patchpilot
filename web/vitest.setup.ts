import "@testing-library/jest-dom/vitest";

import type * as NuqsModule from "nuqs";
import type * as ReactModule from "react";
import { vi } from "vitest";

declare global {
  var __patchPilotNuqsQueryState: Map<string, string>;
}

const nuqsQueryState = vi.hoisted(() => new Map<string, string>());

globalThis.__patchPilotNuqsQueryState = nuqsQueryState;

vi.mock("nuqs", async () => {
  const React = await vi.importActual<typeof ReactModule>("react");
  const actual = await vi.importActual<typeof NuqsModule>("nuqs");

  return {
    ...actual,
    useQueryState: (key: string) => {
      const [value, setValue] = React.useState(nuqsQueryState.get(key) ?? "");
      return [
        value,
        (nextValue: string) => {
          if (nextValue.length > 0) {
            nuqsQueryState.set(key, nextValue);
          } else {
            nuqsQueryState.delete(key);
          }
          setValue(nextValue);
          const searchParams = new URLSearchParams();
          nuqsQueryState.forEach((paramValue, paramKey) => {
            searchParams.set(paramKey, paramValue);
          });
          return Promise.resolve(searchParams);
        },
      ] as const;
    },
  };
});

if (!("hasPointerCapture" in Element.prototype)) {
  Object.defineProperty(Element.prototype, "hasPointerCapture", {
    value: () => false,
  });
}

if (!("releasePointerCapture" in Element.prototype)) {
  Object.defineProperty(Element.prototype, "releasePointerCapture", {
    value: () => undefined,
  });
}

if (!("scrollIntoView" in Element.prototype)) {
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    value: () => undefined,
    writable: true,
  });
}

const testRect = {
  bottom: 0,
  height: 0,
  left: 0,
  right: 0,
  top: 0,
  width: 0,
  x: 0,
  y: 0,
  toJSON: () => ({}),
};

if (!("getBoundingClientRect" in Range.prototype)) {
  Object.defineProperty(Range.prototype, "getBoundingClientRect", {
    configurable: true,
    value: () => testRect,
  });
}

if (!("getClientRects" in Range.prototype)) {
  Object.defineProperty(Range.prototype, "getClientRects", {
    configurable: true,
    value: () => [testRect],
  });
}

if (!("getClientRects" in Text.prototype)) {
  Object.defineProperty(Text.prototype, "getClientRects", {
    configurable: true,
    value: () => [testRect],
  });
}

let testPointElement: Element | null = null;
for (const eventName of [
  "mousedown",
  "mousemove",
  "mouseover",
  "pointermove",
  "pointerover",
]) {
  document.addEventListener(
    eventName,
    (event) => {
      testPointElement = event.target instanceof Element ? event.target : null;
    },
    true,
  );
}

if (!("elementFromPoint" in document)) {
  Object.defineProperty(document, "elementFromPoint", {
    configurable: true,
    value: () =>
      testPointElement ??
      (document.activeElement instanceof Element
        ? document.activeElement
        : document.body),
  });
}

if (!("setPointerCapture" in Element.prototype)) {
  Object.defineProperty(Element.prototype, "setPointerCapture", {
    value: () => undefined,
  });
}
