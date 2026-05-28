import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DiffViewer } from "./diff-viewer";

describe("DiffViewer", () => {
  it("syntax-highlights code inside diff lines", () => {
    const diff = `diff --git a/.commitlintrc.js b/.commitlintrc.js
index 1111111..2222222 100644
--- a/.commitlintrc.js
+++ b/.commitlintrc.js
@@ -1,3 +1,4 @@
-export default { old: true };
+export default { header: 'value' };
`;

    const { container } = render(<DiffViewer diff={diff} />);

    expect(container.querySelector(".token.keyword")).toBeInTheDocument();
    expect(container.querySelector(".token.string")).toBeInTheDocument();
  });

  it("can wrap long patch lines inside constrained tool previews", () => {
    const diff = `diff --git a/web/index.html b/web/index.html
index 1111111..2222222 100644
--- a/web/index.html
+++ b/web/index.html
@@ -1 +1 @@
-<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover, very-long-token-that-would-overflow" />
+<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover, another-very-long-token-that-would-overflow" />
`;

    const { container } = render(<DiffViewer diff={diff} wrapLines />);

    expect(container.querySelector(".break-words")).toBeInTheDocument();
    expect(container.querySelector(".min-w-0")).toBeInTheDocument();
  });

  it("combines single-hunk tool preview headers", () => {
    const diff = `diff --git a/apps/webui/index.html b/apps/webui/index.html
--- a/apps/webui/index.html
+++ b/apps/webui/index.html
@@ -7,10 +7,10 @@
-  <title>Tirtea</title>
+  <title>ACB</title>
`;

    const { container, getByText } = render(
      <DiffViewer diff={diff} wrapLines />,
    );

    expect(getByText("apps/webui/index.html")).toBeInTheDocument();
    expect(getByText("@@ -7,10 +7,10 @@")).toBeInTheDocument();
    expect(container.querySelectorAll("[data-diff-hunk-header]")).toHaveLength(
      0,
    );
  });

  it("collapses and expands file sections", () => {
    const diff = `diff --git a/apps/webui/index.html b/apps/webui/index.html
--- a/apps/webui/index.html
+++ b/apps/webui/index.html
@@ -7,10 +7,10 @@
-  <title>Tirtea</title>
+  <title>ACB</title>
`;

    const { container, getByRole } = render(<DiffViewer diff={diff} />);
    const fileToggle = getByRole("button", {
      name: /apps\/webui\/index\.html/,
    });
    const content = container.querySelector("[data-diff-file-content]");

    expect(fileToggle).toHaveAttribute("aria-expanded", "false");
    expect(content).toHaveAttribute("aria-hidden", "true");

    fireEvent.click(fileToggle);

    expect(fileToggle).toHaveAttribute("aria-expanded", "true");
    expect(content).toHaveAttribute("aria-hidden", "false");

    fireEvent.click(fileToggle);

    expect(fileToggle).toHaveAttribute("aria-expanded", "false");
    expect(content).toHaveAttribute("aria-hidden", "true");
  });
});
