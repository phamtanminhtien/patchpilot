import {
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  apiErrorMessage,
  closeTerminalSession,
  commitGitChanges,
  createTerminalSession,
  createWorkspace,
  discardGitChanges,
  exposePort,
  getGitDiff,
  getGitStatus,
  getWorkspace,
  listFileIndex,
  listPorts,
  listTerminalSessions,
  listWorkspaces,
  patchTerminalSession,
  type Port,
  type PortListResponse,
  readFile,
  refreshFileIndex,
  searchFiles,
  stageGitFiles,
  type TerminalSession,
  type TerminalSessionListResponse,
  unstageGitFiles,
  type WorkspaceEvent,
  writeFile,
} from "@/shared/api";
import { useWorkspaceEvents } from "@/shared/events";

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
  const [selectedTerminal, setSelectedTerminal] = useState({
    sessionId: "",
    workspaceId: "",
  });
  const [fileSearchState, setFileSearchState] = useState({
    query: "",
    workspaceId: "",
  });
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

  const fileSearchQuery =
    fileSearchState.workspaceId === workspaceId ? fileSearchState.query : "";
  const trimmedFileSearchQuery = fileSearchQuery.trim();

  const fileSearchQueryResult = useQuery({
    enabled:
      workspaceId.length > 0 &&
      panel === "files" &&
      trimmedFileSearchQuery.length > 0,
    queryFn: () => searchFiles(workspaceId, trimmedFileSearchQuery),
    queryKey: ["workspace-file-search", workspaceId, trimmedFileSearchQuery],
  });

  const fileQuery = useQuery({
    enabled:
      workspaceId.length > 0 && panel === "files" && selectedPath.length > 0,
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

  const terminalSessionsQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listTerminalSessions(workspaceId),
    queryKey: ["workspace-terminal-sessions", workspaceId],
  });

  const portsQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "preview",
    queryFn: () => listPorts(workspaceId),
    queryKey: ["workspace-ports", workspaceId],
  });

  const selectedTerminalId =
    selectedTerminal.workspaceId === workspaceId
      ? selectedTerminal.sessionId
      : "";
  const visibleTerminalSessions = useMemo(
    () =>
      (terminalSessionsQuery.data?.sessions ?? []).filter(
        (session) => session.status !== "closed",
      ),
    [terminalSessionsQuery.data?.sessions],
  );
  const activeTerminalId = visibleTerminalSessions.some(
    (session) => session.id === selectedTerminalId,
  )
    ? selectedTerminalId
    : (visibleTerminalSessions[0]?.id ?? "");
  const activeTerminal =
    visibleTerminalSessions.find(
      (session) => session.id === activeTerminalId,
    ) ?? null;

  const createTerminalMutation = useMutation({
    mutationFn: () => createTerminalSession(workspaceId),
    onSuccess: (session) => {
      setSelectedTerminal({ sessionId: session.id, workspaceId });
      updateTerminalSessionCache(queryClient, workspaceId, session);
    },
  });

  const renameTerminalMutation = useMutation({
    mutationFn: ({ sessionId, title }: { sessionId: string; title: string }) =>
      patchTerminalSession(workspaceId, sessionId, { title }),
    onSuccess: (session) => {
      updateTerminalSessionCache(queryClient, workspaceId, session);
    },
  });

  const closeTerminalMutation = useMutation({
    mutationFn: (sessionId: string) =>
      closeTerminalSession(workspaceId, sessionId),
    onSuccess: (session) => {
      updateTerminalSessionCache(queryClient, workspaceId, session);
      if (selectedTerminalId === session.id) {
        setSelectedTerminal({ sessionId: "", workspaceId });
      }
    },
  });

  const exposePortMutation = useMutation({
    mutationFn: (port: number) => exposePort(workspaceId, port),
    onSuccess: (port) => {
      updatePortCache(queryClient, workspaceId, port);
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

  const writeFileMutation = useMutation({
    mutationFn: (content: string) =>
      writeFile(workspaceId, { content, path: selectedPath }),
    onSuccess: (file) => {
      queryClient.setQueryData(
        ["workspace-file", workspaceId, file.path],
        file,
      );
      void queryClient.invalidateQueries({
        queryKey: ["workspace-file-index", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-status", workspaceId],
      });
    },
  });

  const resetCommitMutation = commitMutation.reset;
  const resetWriteFileMutation = writeFileMutation.reset;

  useEffect(() => {
    resetWorkspaceScopedState();
    resetCommitMutation();
    resetWriteFileMutation();
  }, [
    resetCommitMutation,
    resetWorkspaceScopedState,
    resetWriteFileMutation,
    workspaceId,
  ]);

  useEffect(() => {
    resetWriteFileMutation();
  }, [resetWriteFileMutation, selectedPath]);

  const handleWorkspaceEvent = useCallback(
    (event: WorkspaceEvent) => {
      if (
        event.type === "port.opened" ||
        event.type === "port.exposed" ||
        event.type === "port.closed"
      ) {
        updatePortCache(queryClient, workspaceId, event.payload as Port);
        return;
      }
      if (event.type === "git.changed") {
        queryClient.setQueryData(
          ["workspace-git-status", workspaceId],
          event.payload,
        );
        return;
      }
      if (
        event.type === "terminal.session.created" ||
        event.type === "terminal.session.updated" ||
        event.type === "terminal.session.closed"
      ) {
        updateTerminalSessionCache(
          queryClient,
          workspaceId,
          event.payload as TerminalSession,
        );
        return;
      }
    },
    [queryClient, workspaceId],
  );

  useWorkspaceEvents(workspaceId, handleWorkspaceEvent);

  function handlePanelChange(nextPanel: WorkspacePanel) {
    void setPanel(nextPanel);
  }

  function handlePathSelect(path: string) {
    void setSelectedPath(path);
  }

  function handleFileSearchQueryChange(query: string) {
    setFileSearchState({ query, workspaceId });
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

  function handleTerminalCreate() {
    if (createTerminalMutation.isPending) {
      return;
    }
    createTerminalMutation.mutate();
  }

  function handleTerminalSelect(sessionId: string) {
    setSelectedTerminal({ sessionId, workspaceId });
  }

  function handleTerminalRename(sessionId: string, title: string) {
    if (title.trim().length === 0 || renameTerminalMutation.isPending) {
      return;
    }
    renameTerminalMutation.mutate({ sessionId, title });
  }

  function handleTerminalClose(sessionId: string) {
    if (closeTerminalMutation.isPending) {
      return;
    }
    closeTerminalMutation.mutate(sessionId);
  }

  return {
    terminal: {
      activeSession: activeTerminal,
      activeSessionId: activeTerminalId,
      closeError: closeTerminalMutation.error
        ? apiErrorMessage(closeTerminalMutation.error)
        : undefined,
      createError: createTerminalMutation.error
        ? apiErrorMessage(createTerminalMutation.error)
        : undefined,
      isClosing: closeTerminalMutation.isPending,
      isCreating: createTerminalMutation.isPending,
      isLoading: terminalSessionsQuery.isPending,
      isRenaming: renameTerminalMutation.isPending,
      onClose: handleTerminalClose,
      onCreate: handleTerminalCreate,
      onRename: handleTerminalRename,
      onSelect: handleTerminalSelect,
      renameError: renameTerminalMutation.error
        ? apiErrorMessage(renameTerminalMutation.error)
        : undefined,
      sessions: visibleTerminalSessions,
    },
    files: {
      entries: filesQuery.data?.entries ?? [],
      error: filesQuery.error ? apiErrorMessage(filesQuery.error) : undefined,
      file: fileQuery.data?.content,
      fileError: fileQuery.error ? apiErrorMessage(fileQuery.error) : undefined,
      isFileLoading: fileQuery.isPending,
      isLoading: filesQuery.isPending,
      isRefreshing: refreshFilesMutation.isPending,
      isSaving: writeFileMutation.isPending,
      onRefresh: () => refreshFilesMutation.mutate(),
      onSave: (content: string) => writeFileMutation.mutate(content),
      onSearchQueryChange: handleFileSearchQueryChange,
      saveError: writeFileMutation.error
        ? apiErrorMessage(writeFileMutation.error)
        : undefined,
      searchError: fileSearchQueryResult.error
        ? apiErrorMessage(fileSearchQueryResult.error)
        : undefined,
      searchQuery: fileSearchQuery,
      searchResults: fileSearchQueryResult.data?.results ?? [],
      searchTrimmedQuery: trimmedFileSearchQuery,
      isSearching: fileSearchQueryResult.isFetching,
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
      stagedPathsForCommit: stagedGitPaths,
      stageError: stageMutation.error
        ? apiErrorMessage(stageMutation.error)
        : undefined,
      unstagedPathCount: unstagedGitPaths.length,
    },
    preview: {
      error: portsQuery.error ? apiErrorMessage(portsQuery.error) : undefined,
      exposeError: exposePortMutation.error
        ? apiErrorMessage(exposePortMutation.error)
        : undefined,
      exposingPort: exposePortMutation.variables,
      isExposing: exposePortMutation.isPending,
      isLoading: portsQuery.isPending,
      onExpose: (port: number) => exposePortMutation.mutate(port),
      ports: portsQuery.data?.ports ?? [],
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

function updateTerminalSessionCache(
  queryClient: QueryClient,
  workspaceId: string,
  session: TerminalSession,
) {
  queryClient.setQueryData<TerminalSessionListResponse>(
    ["workspace-terminal-sessions", workspaceId],
    (current) => ({
      sessions: [
        session,
        ...(current?.sessions.filter((item) => item.id !== session.id) ?? []),
      ],
    }),
  );
}

function updatePortCache(
  queryClient: QueryClient,
  workspaceId: string,
  port: Port,
) {
  queryClient.setQueryData<PortListResponse>(
    ["workspace-ports", workspaceId],
    (current) => ({
      ports: [
        port,
        ...(current?.ports.filter((item) => item.id !== port.id) ?? []),
      ].sort((a, b) => a.port - b.port),
    }),
  );
}
