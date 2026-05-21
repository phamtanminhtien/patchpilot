import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useMemo } from "react";

import {
  apiErrorMessage,
  commitGitChanges,
  createWorkspace,
  discardGitChanges,
  getGitDiff,
  getGitStatus,
  getWorkspace,
  listFileIndex,
  listWorkspaces,
  queueCommand,
  readFile,
  refreshFileIndex,
  stageGitFiles,
  unstageGitFiles,
} from "@/shared/api";

import {
  parseGitPorcelain,
  stagedGitPaths as selectStagedGitPaths,
  unstagedGitPaths as selectUnstagedGitPaths,
} from "../git/workspace-git";
import type { WorkspacePanel } from "../workspace-panels";
import { useWorkspaceUiStore } from "./use-workspace-ui-store";

interface UseWorkspaceControllerInput {
  panel: WorkspacePanel;
  selectedPath: string;
  setPanel: (panel: WorkspacePanel) => Promise<URLSearchParams>;
  setSelectedPath: (path: string) => Promise<URLSearchParams>;
  setWorkspaceId: (workspaceId: string) => Promise<URLSearchParams>;
  workspaceId: string;
}

export function useWorkspaceController({
  panel,
  selectedPath,
  setPanel,
  setSelectedPath,
  setWorkspaceId,
  workspaceId,
}: UseWorkspaceControllerInput) {
  const queryClient = useQueryClient();
  const starterRootPath = useWorkspaceUiStore((state) => state.starterRootPath);
  const setStarterRootPath = useWorkspaceUiStore(
    (state) => state.setStarterRootPath,
  );
  const commandText = useWorkspaceUiStore((state) => state.commandText);
  const setCommandText = useWorkspaceUiStore((state) => state.setCommandText);
  const commitMessage = useWorkspaceUiStore((state) => state.commitMessage);
  const setCommitMessage = useWorkspaceUiStore(
    (state) => state.setCommitMessage,
  );
  const resetStarterState = useWorkspaceUiStore(
    (state) => state.resetStarterState,
  );
  const resetWorkspaceScopedState = useWorkspaceUiStore(
    (state) => state.resetWorkspaceScopedState,
  );

  const workspaceQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspace(workspaceId),
    queryKey: ["workspace", workspaceId],
  });

  const workspacesQuery = useQuery({
    enabled: workspaceId.length === 0,
    queryFn: listWorkspaces,
    queryKey: ["workspaces"],
  });

  const createWorkspaceMutation = useMutation({
    mutationFn: createWorkspace,
    onSuccess: (workspace) => {
      resetStarterState();
      void setWorkspaceId(workspace.id);
    },
  });

  const filesQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "files",
    queryFn: () => listFileIndex(workspaceId),
    queryKey: ["workspace-file-index", workspaceId],
  });

  const selectedFileEntry = filesQuery.data?.entries.find(
    (entry) => entry.path === selectedPath,
  );

  const fileQuery = useQuery({
    enabled:
      workspaceId.length > 0 &&
      panel === "files" &&
      selectedPath.length > 0 &&
      selectedFileEntry !== undefined,
    queryFn: () => readFile(workspaceId, selectedPath),
    queryKey: ["workspace-file", workspaceId, selectedPath],
  });

  const gitQuery = useQuery({
    enabled: workspaceId.length > 0 && (panel === "files" || panel === "git"),
    queryFn: () => getGitStatus(workspaceId),
    queryKey: ["workspace-git-status", workspaceId],
  });

  const gitChanges = useMemo(
    () => parseGitPorcelain(gitQuery.data?.porcelain ?? ""),
    [gitQuery.data?.porcelain],
  );

  const stagedGitPaths = useMemo(
    () => selectStagedGitPaths(gitChanges),
    [gitChanges],
  );

  const unstagedGitPaths = useMemo(
    () => selectUnstagedGitPaths(gitChanges),
    [gitChanges],
  );

  const gitDiffQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "git",
    queryFn: () => getGitDiff(workspaceId, selectedPath || undefined),
    queryKey: ["workspace-git-diff", workspaceId, selectedPath],
  });

  const commandMutation = useMutation({
    mutationFn: (command: string) => queueCommand(workspaceId, command),
    onSuccess: () => {
      setCommandText("");
    },
  });

  const commitMutation = useMutation({
    mutationFn: (paths: string[]) =>
      commitGitChanges(workspaceId, { message: commitMessage, paths }),
    onSuccess: () => {
      setCommitMessage("");
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-status", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
    },
  });

  const stageMutation = useMutation({
    mutationFn: (paths: string[]) => stageGitFiles(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      commitMutation.reset();
    },
  });

  const unstageMutation = useMutation({
    mutationFn: (paths: string[]) => unstageGitFiles(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      commitMutation.reset();
    },
  });

  const discardMutation = useMutation({
    mutationFn: (paths: string[]) => discardGitChanges(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      commitMutation.reset();
    },
  });

  const refreshFilesMutation = useMutation({
    mutationFn: () => refreshFileIndex(workspaceId),
    onSuccess: (data) => {
      queryClient.setQueryData(["workspace-file-index", workspaceId], data);
    },
  });

  const resetCommandMutation = commandMutation.reset;
  const resetCommitMutation = commitMutation.reset;

  useEffect(() => {
    resetWorkspaceScopedState();
    resetCommandMutation();
    resetCommitMutation();
  }, [
    resetCommandMutation,
    resetCommitMutation,
    resetWorkspaceScopedState,
    workspaceId,
  ]);

  function handlePanelChange(nextPanel: WorkspacePanel) {
    void setPanel(nextPanel);
  }

  function handlePathSelect(path: string) {
    void setSelectedPath(path);
  }

  function handleWorkspaceSelect(selectedWorkspaceId: string) {
    void setWorkspaceId(selectedWorkspaceId);
  }

  function handleWorkspaceCreate() {
    createWorkspaceMutation.mutate(starterRootPath);
  }

  function handleStageChanges() {
    if (unstagedGitPaths.length === 0 || stageMutation.isPending) {
      return;
    }
    stageMutation.mutate(unstagedGitPaths);
  }

  function handleStageSelectedChanges(paths: string[]) {
    if (paths.length === 0 || stageMutation.isPending) {
      return;
    }
    stageMutation.mutate(paths);
  }

  function handleUnstageChanges(paths: string[]) {
    if (paths.length === 0 || unstageMutation.isPending) {
      return;
    }
    unstageMutation.mutate(paths);
  }

  function handleDiscardChanges(paths: string[]) {
    if (paths.length === 0 || discardMutation.isPending) {
      return;
    }
    discardMutation.mutate(paths);
  }

  function handleCommitSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      stagedGitPaths.length === 0 ||
      commitMessage.trim().length === 0 ||
      commitMutation.isPending
    ) {
      return;
    }
    commitMutation.mutate(stagedGitPaths);
  }

  function handleCommandSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const command = commandText.trim();
    if (command.length === 0 || commandMutation.isPending) {
      return;
    }
    commandMutation.mutate(command);
  }

  return {
    command: {
      error: commandMutation.error
        ? apiErrorMessage(commandMutation.error)
        : undefined,
      isPending: commandMutation.isPending,
      onCommandChange: setCommandText,
      onSubmit: handleCommandSubmit,
      queuedCommand: commandMutation.data ?? null,
      text: commandText,
    },
    files: {
      entries: filesQuery.data?.entries ?? [],
      error: filesQuery.error ? apiErrorMessage(filesQuery.error) : undefined,
      file: fileQuery.data?.content,
      fileError: fileQuery.error ? apiErrorMessage(fileQuery.error) : undefined,
      isFileLoading: fileQuery.isPending,
      isLoading: filesQuery.isPending,
      isRefreshing: refreshFilesMutation.isPending,
      onRefresh: () => refreshFilesMutation.mutate(),
    },
    git: {
      changes: gitChanges,
      commitError: commitMutation.error
        ? apiErrorMessage(commitMutation.error)
        : undefined,
      commitMessage,
      diff: gitDiffQuery.data?.diff,
      diffError: gitDiffQuery.error
        ? apiErrorMessage(gitDiffQuery.error)
        : undefined,
      error: gitQuery.error ? apiErrorMessage(gitQuery.error) : undefined,
      isCommitPending: commitMutation.isPending,
      isDiscardingChanges: discardMutation.isPending,
      isDiffLoading: gitDiffQuery.isPending,
      isLoading: gitQuery.isPending,
      isStagingChanges: stageMutation.isPending,
      isUnstagingChanges: unstageMutation.isPending,
      lastCommitHash: commitMutation.data?.hash,
      onChangesDiscard: handleDiscardChanges,
      onChangesStage: handleStageSelectedChanges,
      onCommitMessageChange: setCommitMessage,
      onCommitSubmit: handleCommitSubmit,
      onStageChanges: handleStageChanges,
      onStagedChangesUnstage: handleUnstageChanges,
      rawStatus: gitQuery.data?.porcelain,
      stagedPathCount: stagedGitPaths.length,
      stageError: stageMutation.error
        ? apiErrorMessage(stageMutation.error)
        : undefined,
      unstagedPathCount: unstagedGitPaths.length,
    },
    starter: {
      createError: createWorkspaceMutation.error
        ? apiErrorMessage(createWorkspaceMutation.error)
        : undefined,
      isCreating: createWorkspaceMutation.isPending,
      isLoadingRecent: workspacesQuery.isPending,
      onRootPathChange: setStarterRootPath,
      onSelectWorkspace: handleWorkspaceSelect,
      onSubmit: handleWorkspaceCreate,
      recentError: workspacesQuery.error
        ? apiErrorMessage(workspacesQuery.error)
        : undefined,
      recentWorkspaces: workspacesQuery.data?.workspaces ?? [],
      rootPath: starterRootPath,
    },
    workspace: {
      data: workspaceQuery.data,
      error: workspaceQuery.error
        ? apiErrorMessage(workspaceQuery.error)
        : undefined,
      onPanelChange: handlePanelChange,
      onPathSelect: handlePathSelect,
    },
  };
}
