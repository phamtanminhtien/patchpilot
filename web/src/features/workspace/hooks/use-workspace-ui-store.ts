import { create } from "zustand";

interface WorkspaceUiState {
  commitMessage: string;
  resetStarterState: () => void;
  resetWorkspaceScopedState: () => void;
  setCommitMessage: (commitMessage: string) => void;
  setStarterRootPath: (starterRootPath: string) => void;
  starterRootPath: string;
}

export const useWorkspaceUiStore = create<WorkspaceUiState>((set) => ({
  commitMessage: "",
  resetStarterState: () => set({ starterRootPath: "" }),
  resetWorkspaceScopedState: () =>
    set({
      commitMessage: "",
    }),
  setCommitMessage: (commitMessage) => set({ commitMessage }),
  setStarterRootPath: (starterRootPath) => set({ starterRootPath }),
  starterRootPath: "",
}));
