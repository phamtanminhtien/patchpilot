import { create } from "zustand";

interface WorkspaceUiState {
  commandText: string;
  commitMessage: string;
  resetStarterState: () => void;
  resetWorkspaceScopedState: () => void;
  setCommandText: (commandText: string) => void;
  setCommitMessage: (commitMessage: string) => void;
  setStarterRootPath: (starterRootPath: string) => void;
  starterRootPath: string;
}

export const useWorkspaceUiStore = create<WorkspaceUiState>((set) => ({
  commandText: "",
  commitMessage: "",
  resetStarterState: () => set({ starterRootPath: "" }),
  resetWorkspaceScopedState: () =>
    set({
      commandText: "",
      commitMessage: "",
    }),
  setCommandText: (commandText) => set({ commandText }),
  setCommitMessage: (commitMessage) => set({ commitMessage }),
  setStarterRootPath: (starterRootPath) => set({ starterRootPath }),
  starterRootPath: "",
}));
