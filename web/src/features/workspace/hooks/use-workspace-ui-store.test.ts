import { beforeEach, describe, expect, it } from "vitest";

import { useWorkspaceUiStore } from "./use-workspace-ui-store";

describe("useWorkspaceUiStore editor sessions", () => {
  beforeEach(() => {
    useWorkspaceUiStore.setState({
      commitMessage: "",
      editorSessions: {},
      starterRootPath: "",
    });
  });

  it("keeps editor tabs isolated by workspace", () => {
    useWorkspaceUiStore.getState().openEditorTab("ws_1", "README.md");
    useWorkspaceUiStore.getState().openEditorTab("ws_2", "docs/spec.md");

    expect(
      useWorkspaceUiStore.getState().editorSessions.ws_1?.openTabs,
    ).toEqual(["README.md"]);
    expect(
      useWorkspaceUiStore.getState().editorSessions.ws_2?.openTabs,
    ).toEqual(["docs/spec.md"]);
  });

  it("preserves a dirty draft when loaded source refreshes", () => {
    const store = useWorkspaceUiStore.getState();
    store.syncEditorFile("ws_1", "README.md", "original");
    useWorkspaceUiStore.getState().setEditorDraft("ws_1", "README.md", "draft");
    useWorkspaceUiStore
      .getState()
      .syncEditorFile("ws_1", "README.md", "remote");

    const session = useWorkspaceUiStore.getState().editorSessions.ws_1;
    expect(session).toBeDefined();
    if (!session) {
      return;
    }
    expect(session.drafts["README.md"]).toBe("draft");
    expect(session.sources["README.md"]).toBe("remote");
  });

  it("clears dirty state when a file is marked saved", () => {
    const store = useWorkspaceUiStore.getState();
    store.syncEditorFile("ws_1", "README.md", "original");
    useWorkspaceUiStore.getState().setEditorDraft("ws_1", "README.md", "draft");
    useWorkspaceUiStore
      .getState()
      .markEditorFileSaved("ws_1", "README.md", "draft");

    const session = useWorkspaceUiStore.getState().editorSessions.ws_1;
    expect(session).toBeDefined();
    if (!session) {
      return;
    }
    expect(session.drafts["README.md"]).toBe("draft");
    expect(session.sources["README.md"]).toBe("draft");
  });
});
