import { describe, expect, it } from "vitest";

import { resolveFileIconName } from "./file-icon";

describe("resolveFileIconName", () => {
  it("uses folder icons with opened variants", () => {
    expect(
      resolveFileIconName({
        isDirectory: true,
        isExpanded: true,
        path: "web/src",
      }),
    ).toBe("folder_type_src_opened");
  });

  it("uses specific file name icons before extension icons", () => {
    expect(resolveFileIconName({ path: "web/package.json" })).toBe(
      "file_type_node",
    );
  });

  it("falls back to extension icons for regular files", () => {
    expect(resolveFileIconName({ path: "web/src/main.tsx" })).toBe(
      "file_type_reactts",
    );
  });
});
