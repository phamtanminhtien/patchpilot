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

if (!("setPointerCapture" in Element.prototype)) {
  Object.defineProperty(Element.prototype, "setPointerCapture", {
    value: () => undefined,
  });
}
