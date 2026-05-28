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
  type FileIndexEntry,
  getGitDiff,
  getGitStatus,
  getWorkspace,
  listFileIndexDirectory,
  listGitBranches,
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
  stageGitPatch,
  switchGitBranch,
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

const defaultContentSearchExclude =
  ".git/**, **/.git/**, node_modules/**, **/node_modules/**, .pnpm/**, **/.pnpm/**, .yarn/**, **/.yarn/**, .next/**, **/.next/**, .nuxt/**, **/.nuxt/**, dist/**, **/dist/**, build/**, **/build/**, coverage/**, **/coverage/**, .cache/**, **/.cache/**, .turbo/**, **/.turbo/**, .vite/**, **/.vite/**";

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
    exclude: defaultContentSearchExclude,
    include: "",
    query: "",
    workspaceId: "",
  });
  const commitMessage = useWorkspaceUiStore((state) => state.commitMessage);
  const closeEditorTab = useWorkspaceUiStore((state) => state.closeEditorTab);
  const editorSessions = useWorkspaceUiStore((state) => state.editorSessions);
  const markEditorFileSaved = useWorkspaceUiStore(
    (state) => state.markEditorFileSaved,
  );
  const openEditorTab = useWorkspaceUiStore((state) => state.openEditorTab);
  const setEditorDraft = useWorkspaceUiStore((state) => state.setEditorDraft);
  const setCommitMessage = useWorkspaceUiStore(
    (state) => state.setCommitMessage,
  );
  const syncEditorFile = useWorkspaceUiStore((state) => state.syncEditorFile);
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
    queryFn: () =>
      listFileIndexDirectory(workspaceId, "", { includeSkipped: true }),
    queryKey: ["workspace-file-index", workspaceId],
  });

  const fileSearchQuery =
    fileSearchState.workspaceId === workspaceId ? fileSearchState.query : "";
  const fileSearchInclude =
    fileSearchState.workspaceId === workspaceId ? fileSearchState.include : "";
  const fileSearchExclude =
    fileSearchState.workspaceId === workspaceId
      ? fileSearchState.exclude
      : defaultContentSearchExclude;
  const trimmedFileSearchQuery = fileSearchQuery.trim();
  const trimmedFileSearchInclude = fileSearchInclude.trim();
  const trimmedFileSearchExclude = fileSearchExclude.trim();

  const fileSearchQueryResult = useQuery({
    enabled:
      workspaceId.length > 0 &&
      panel === "search" &&
      trimmedFileSearchQuery.length > 0,
    queryFn: () =>
      searchFiles(workspaceId, trimmedFileSearchQuery, {
        exclude: trimmedFileSearchExclude || undefined,
        include: trimmedFileSearchInclude || undefined,
      }),
    queryKey: [
      "workspace-file-search",
      workspaceId,
      trimmedFileSearchQuery,
      trimmedFileSearchInclude,
      trimmedFileSearchExclude,
    ],
  });

  const fileQuery = useQuery({
    enabled: workspaceId.length > 0 && selectedPath.length > 0,
    queryFn: () => readFile(workspaceId, selectedPath),
    queryKey: ["workspace-file", workspaceId, selectedPath],
  });

  const gitQuery = useQuery({
    enabled:
      workspaceId.length > 0 &&
      (panel === "files" || panel === "search" || panel === "git"),
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

  const gitBranchesQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listGitBranches(workspaceId),
    queryKey: ["workspace-git-branches", workspaceId],
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

  const stagePatchMutation = useMutation({
    mutationFn: (patch: string) =>
      stageGitPatch(workspaceId, { direction: "forward", patch }),
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

  const switchBranchMutation = useMutation({
    mutationFn: (branch: string) => switchGitBranch(workspaceId, { branch }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-branches", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-file-index", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-file-index-directory", workspaceId],
      });
    },
  });

  const refreshFilesMutation = useMutation({
    mutationFn: () => refreshFileIndex(workspaceId),
    onSuccess: (data) => {
      queryClient.setQueryData(["workspace-file-index", workspaceId], {
        ...data,
        entries: data.entries.filter(isRootFileIndexEntry),
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-file-index-directory", workspaceId],
      });
    },
  });

  const writeFileMutation = useMutation({
    mutationFn: ({ content, path }: { content: string; path: string }) =>
      writeFile(workspaceId, { content, path }),
    onSuccess: (file) => {
      queryClient.setQueryData(
        ["workspace-file", workspaceId, file.path],
        file,
      );
      markEditorFileSaved(workspaceId, file.path, file.content);
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

  useEffect(() => {
    if (
      workspaceId.length === 0 ||
      selectedPath.length === 0 ||
      fileQuery.data === undefined
    ) {
      return;
    }
    syncEditorFile(workspaceId, fileQuery.data.path, fileQuery.data.content);
  }, [fileQuery.data, selectedPath, syncEditorFile, workspaceId]);

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
    openEditorTab(workspaceId, path);
    void setSelectedPath(path);
  }

  function handleFileOpen(path: string) {
    openEditorTab(workspaceId, path);
    void setSelectedPath(path);
    void setPanel("files");
  }

  function handleSearchResultOpen(path: string) {
    openEditorTab(workspaceId, path);
    void setSelectedPath(path);
  }

  function handleEditorTabClose(path: string) {
    const session = editorSessions[workspaceId];
    const openTabs = session?.openTabs ?? [];
    const closingIndex = openTabs.indexOf(path);
    const remainingTabs = openTabs.filter((tabPath) => tabPath !== path);
    closeEditorTab(workspaceId, path);

    if (path === selectedPath) {
      const nextPath =
        remainingTabs[Math.min(closingIndex, remainingTabs.length - 1)] ?? "";
      void setSelectedPath(nextPath);
    }
  }

  function handleFileSearchQueryChange(query: string) {
    setFileSearchState({
      exclude: fileSearchExclude,
      include: fileSearchInclude,
      query,
      workspaceId,
    });
  }

  function handleFileSearchIncludeChange(include: string) {
    setFileSearchState({
      exclude: fileSearchExclude,
      include,
      query: fileSearchQuery,
      workspaceId,
    });
  }

  function handleFileSearchExcludeChange(exclude: string) {
    setFileSearchState({
      exclude,
      include: fileSearchInclude,
      query: fileSearchQuery,
      workspaceId,
    });
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

  function handleStagePatch(patch: string) {
    if (patch.trim().length === 0 || stagePatchMutation.isPending) {
      return;
    }
    stagePatchMutation.mutate(patch);
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

  function handleFileSave(path: string, content: string) {
    if (path.length === 0 || writeFileMutation.isPending) {
      return;
    }
    writeFileMutation.mutate({ content, path });
  }

  function handleFileSaveAll() {
    if (writeFileMutation.isPending) {
      return;
    }
    const session = editorSessions[workspaceId];
    if (!session) {
      return;
    }
    for (const path of session.openTabs) {
      const draft = session.drafts[path];
      const source = session.sources[path];
      if (draft !== undefined && source !== undefined && draft !== source) {
        writeFileMutation.mutate({ content: draft, path });
      }
    }
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

  const editorSession = editorSessions[workspaceId] ?? {
    drafts: {},
    openTabs: [],
    sources: {},
  };
  const dirtyPaths = editorSession.openTabs.filter((path) => {
    const draft = editorSession.drafts[path];
    const source = editorSession.sources[path];
    return draft !== undefined && source !== undefined && draft !== source;
  });
  const activeDraft = selectedPath
    ? (editorSession.drafts[selectedPath] ?? fileQuery.data?.content ?? "")
    : "";
  const activeSource = selectedPath
    ? (editorSession.sources[selectedPath] ?? fileQuery.data?.content ?? "")
    : "";

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
      activeDraft,
      activeSource,
      dirtyPaths,
      isFileLoading: fileQuery.isPending,
      isLoading: filesQuery.isPending,
      isRefreshing: refreshFilesMutation.isPending,
      isSaving: writeFileMutation.isPending,
      openTabs: editorSession.openTabs,
      onCloseTab: handleEditorTabClose,
      onDraftChange: (path: string, content: string) =>
        setEditorDraft(workspaceId, path, content),
      onRefresh: () => refreshFilesMutation.mutate(),
      onSave: handleFileSave,
      onSaveAll: handleFileSaveAll,
      onSearchExcludeChange: handleFileSearchExcludeChange,
      onSearchIncludeChange: handleFileSearchIncludeChange,
      onSearchResultOpen: handleSearchResultOpen,
      onSearchQueryChange: handleFileSearchQueryChange,
      onOpenFile: handleFileOpen,
      onSelectTab: handlePathSelect,
      saveError: writeFileMutation.error
        ? apiErrorMessage(writeFileMutation.error)
        : undefined,
      searchError: fileSearchQueryResult.error
        ? apiErrorMessage(fileSearchQueryResult.error)
        : undefined,
      searchExclude: fileSearchExclude,
      searchInclude: fileSearchInclude,
      searchQuery: fileSearchQuery,
      searchResults: fileSearchQueryResult.data?.results ?? [],
      searchTrimmedQuery: trimmedFileSearchQuery,
      isSearching: fileSearchQueryResult.isFetching,
    },
    git: {
      author: gitQuery.data?.author,
      branch: gitQuery.data?.branch,
      branches: gitBranchesQuery.data?.branches ?? [],
      branchesError: gitBranchesQuery.error
        ? apiErrorMessage(gitBranchesQuery.error)
        : undefined,
      changesCount: gitChanges.length,
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
      head: gitQuery.data?.head,
      isCommitPending: commitMutation.isPending,
      isDiscardingChanges: discardMutation.isPending,
      isDiffLoading: gitDiffQuery.isPending,
      isLoading: gitQuery.isPending,
      isBranchListLoading: gitBranchesQuery.isPending,
      isSwitchingBranch: switchBranchMutation.isPending,
      isPatchStaging: stagePatchMutation.isPending,
      isStagingChanges: stageMutation.isPending,
      isUnstagingChanges: unstageMutation.isPending,
      lastCommitHash: commitMutation.data?.hash,
      onChangesDiscard: handleDiscardChanges,
      onChangesStage: handleStageSelectedChanges,
      onCommitMessageChange: setCommitMessage,
      onCommitSubmit: handleCommitSubmit,
      onStageChanges: handleStageChanges,
      onStagePatch: handleStagePatch,
      onStagedChangesUnstage: handleUnstageChanges,
      onSwitchBranch: (branch: string) =>
        switchBranchMutation.mutateAsync(branch),
      rawStatus: gitQuery.data?.porcelain,
      stagedPathCount: stagedGitPaths.length,
      stagedPathsForCommit: stagedGitPaths,
      stageError:
        stageMutation.error || stagePatchMutation.error
          ? apiErrorMessage(stageMutation.error ?? stagePatchMutation.error)
          : undefined,
      switchBranchError: switchBranchMutation.error
        ? apiErrorMessage(switchBranchMutation.error)
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

function isRootFileIndexEntry(entry: FileIndexEntry) {
  if (entry.dir !== undefined) {
    return entry.dir === "";
  }
  return !entry.path.includes("/");
}
