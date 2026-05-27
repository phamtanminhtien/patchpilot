import { create } from "zustand";

interface EditorSessionState {
  drafts: Record<string, string>;
  openTabs: string[];
  sources: Record<string, string>;
}

interface WorkspaceUiState {
  commitMessage: string;
  closeEditorTab: (workspaceId: string, path: string) => void;
  editorSessions: Record<string, EditorSessionState>;
  markEditorFileSaved: (
    workspaceId: string,
    path: string,
    content: string,
  ) => void;
  openEditorTab: (workspaceId: string, path: string) => void;
  resetStarterState: () => void;
  resetWorkspaceScopedState: () => void;
  setEditorDraft: (workspaceId: string, path: string, content: string) => void;
  setCommitMessage: (commitMessage: string) => void;
  setStarterRootPath: (starterRootPath: string) => void;
  starterRootPath: string;
  syncEditorFile: (workspaceId: string, path: string, content: string) => void;
}

function sessionFor(
  sessions: Record<string, EditorSessionState>,
  workspaceId: string,
) {
  return (
    sessions[workspaceId] ?? {
      drafts: {},
      openTabs: [],
      sources: {},
    }
  );
}

function withOpenTab(session: EditorSessionState, path: string) {
  return session.openTabs.includes(path)
    ? session.openTabs
    : [...session.openTabs, path];
}

export const useWorkspaceUiStore = create<WorkspaceUiState>((set) => ({
  closeEditorTab: (workspaceId, path) =>
    set((state) => {
      const session = sessionFor(state.editorSessions, workspaceId);
      const { [path]: _draft, ...drafts } = session.drafts;
      const { [path]: _source, ...sources } = session.sources;

      return {
        editorSessions: {
          ...state.editorSessions,
          [workspaceId]: {
            drafts,
            openTabs: session.openTabs.filter((tabPath) => tabPath !== path),
            sources,
          },
        },
      };
    }),
  commitMessage: "",
  editorSessions: {},
  markEditorFileSaved: (workspaceId, path, content) =>
    set((state) => {
      const session = sessionFor(state.editorSessions, workspaceId);

      return {
        editorSessions: {
          ...state.editorSessions,
          [workspaceId]: {
            drafts: { ...session.drafts, [path]: content },
            openTabs: withOpenTab(session, path),
            sources: { ...session.sources, [path]: content },
          },
        },
      };
    }),
  openEditorTab: (workspaceId, path) =>
    set((state) => {
      if (workspaceId.length === 0 || path.length === 0) {
        return state;
      }
      const session = sessionFor(state.editorSessions, workspaceId);

      return {
        editorSessions: {
          ...state.editorSessions,
          [workspaceId]: {
            ...session,
            openTabs: withOpenTab(session, path),
          },
        },
      };
    }),
  resetStarterState: () => set({ starterRootPath: "" }),
  resetWorkspaceScopedState: () =>
    set({
      commitMessage: "",
    }),
  setCommitMessage: (commitMessage) => set({ commitMessage }),
  setEditorDraft: (workspaceId, path, content) =>
    set((state) => {
      const session = sessionFor(state.editorSessions, workspaceId);

      return {
        editorSessions: {
          ...state.editorSessions,
          [workspaceId]: {
            ...session,
            drafts: { ...session.drafts, [path]: content },
            openTabs: withOpenTab(session, path),
          },
        },
      };
    }),
  setStarterRootPath: (starterRootPath) => set({ starterRootPath }),
  starterRootPath: "",
  syncEditorFile: (workspaceId, path, content) =>
    set((state) => {
      const session = sessionFor(state.editorSessions, workspaceId);
      const currentDraft = session.drafts[path];
      const currentSource = session.sources[path];
      const isDirty =
        currentDraft !== undefined &&
        currentSource !== undefined &&
        currentDraft !== currentSource;

      return {
        editorSessions: {
          ...state.editorSessions,
          [workspaceId]: {
            drafts: {
              ...session.drafts,
              [path]: isDirty ? currentDraft : content,
            },
            openTabs: withOpenTab(session, path),
            sources: { ...session.sources, [path]: content },
          },
        },
      };
    }),
}));

export type { EditorSessionState };
