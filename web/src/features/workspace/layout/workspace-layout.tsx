import { Check, GitBranch as GitBranchIcon } from "lucide-react";
import type { KeyboardEvent as ReactKeyboardEvent, ReactNode } from "react";
import { useMemo, useState } from "react";

import type { GitBranch, Workspace } from "@/shared/api";
import {
  cn,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  TextField,
} from "@/shared/ui";

interface WorkspaceStatusGit {
  author?: string;
  branch?: string;
  branches: GitBranch[];
  branchesError?: string;
  changesCount: number;
  head?: string;
  isBranchListLoading: boolean;
  isLoading: boolean;
  isSwitchingBranch: boolean;
  onSwitchBranch: (branch: string) => Promise<unknown>;
  stagedPathCount: number;
  switchBranchError?: string;
}

export function WorkspaceLayout({
  activityRail,
  bottomPanel,
  git,
  mainPanels,
  sidebar,
  workspace,
}: {
  activityRail: ReactNode;
  bottomPanel: ReactNode;
  git: WorkspaceStatusGit;
  mainPanels: ReactNode;
  sidebar: ReactNode;
  workspace?: Workspace;
}) {
  return (
    <section className="grid h-[calc(100vh-2.75rem)] min-h-0 grid-rows-[minmax(0,1fr)_1.5rem] overflow-hidden">
      <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:grid-cols-[3.25rem_16.5rem_minmax(0,1fr)] lg:grid-rows-1">
        {activityRail}

        <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:contents">
          {sidebar}

          <main className="grid min-h-0 grid-rows-[minmax(0,1fr)_14rem] overflow-hidden lg:grid-rows-[minmax(0,1fr)_16rem]">
            {mainPanels}
            {bottomPanel}
          </main>
        </div>
      </div>
      <WorkspaceStatusBar git={git} workspace={workspace} />
    </section>
  );
}

function WorkspaceStatusBar({
  git,
  workspace,
}: {
  git: WorkspaceStatusGit;
  workspace?: Workspace;
}) {
  const [isBranchDialogOpen, setIsBranchDialogOpen] = useState(false);
  const author = git.author?.trim() || "Unknown author";
  const branch = git.branch?.trim() || "detached";
  const head = git.head?.trim();
  const repoName = workspace?.name ?? "Workspace";
  const changeSummary = git.isLoading
    ? "loading git"
    : `${git.changesCount} changed / ${git.stagedPathCount} staged`;

  return (
    <footer className="border-line/55 bg-panel text-muted grid min-h-6 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-t px-2 text-[11px] leading-none sm:px-3">
      <div className="flex min-w-0 items-center gap-2 overflow-hidden font-mono">
        <span className="text-ink max-w-36 truncate" title={repoName}>
          {repoName}
        </span>
        <button
          aria-label={`Switch branch ${branch}`}
          className="bg-surface text-ink hover:bg-hover inline-flex max-w-32 cursor-pointer items-center gap-1 truncate rounded-sm px-1.5 py-0.5 transition"
          onClick={() => setIsBranchDialogOpen(true)}
          type="button"
        >
          <GitBranchIcon aria-hidden="true" className="size-3 shrink-0" />
          <span className="min-w-0 truncate">{branch}</span>
        </button>
        {head ? <span className="hidden sm:inline">{head}</span> : null}
        <span className="hidden sm:inline">{changeSummary}</span>
      </div>
      <div
        className="flex min-w-0 items-center justify-end gap-1.5 truncate text-right"
        title={author}
      >
        <GitBranchIcon aria-hidden="true" className="size-3 shrink-0" />
        <span className="text-ink min-w-0 truncate">{author}</span>
      </div>
      <BranchSwitchDialog
        currentBranch={branch}
        error={git.branchesError ?? git.switchBranchError}
        isLoading={git.isBranchListLoading}
        isOpen={isBranchDialogOpen}
        isSwitching={git.isSwitchingBranch}
        onOpenChange={setIsBranchDialogOpen}
        onSwitchBranch={git.onSwitchBranch}
        branches={git.branches}
      />
    </footer>
  );
}

function BranchSwitchDialog({
  branches,
  currentBranch,
  error,
  isLoading,
  isOpen,
  isSwitching,
  onOpenChange,
  onSwitchBranch,
}: {
  branches: GitBranch[];
  currentBranch: string;
  error?: string;
  isLoading: boolean;
  isOpen: boolean;
  isSwitching: boolean;
  onOpenChange: (open: boolean) => void;
  onSwitchBranch: (branch: string) => Promise<unknown>;
}) {
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const filteredBranches = useMemo(() => {
    const trimmedQuery = query.trim().toLowerCase();
    if (trimmedQuery.length === 0) {
      return branches;
    }
    return branches.filter((branch) =>
      branch.name.toLowerCase().includes(trimmedQuery),
    );
  }, [branches, query]);
  const activeBranch = filteredBranches[activeIndex];

  async function handleSwitch(branch: string) {
    if (branch === currentBranch || isSwitching) {
      return;
    }
    try {
      await onSwitchBranch(branch);
      onOpenChange(false);
    } catch {
      // Mutation state carries the visible error message.
    }
  }

  function handleInputKeyDown(event: ReactKeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) =>
        Math.min(index + 1, Math.max(filteredBranches.length - 1, 0)),
      );
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(index - 1, 0));
    }
    if (event.key === "Enter") {
      event.preventDefault();
      if (activeBranch) {
        void handleSwitch(activeBranch.name);
      }
    }
  }

  return (
    <DialogRoot
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen);
        setActiveIndex(0);
        if (!nextOpen) {
          setQuery("");
        }
      }}
      open={isOpen}
    >
      <DialogContent
        className="top-8 max-h-[min(34rem,calc(100vh-4rem))] w-[calc(100vw-2rem)] max-w-3xl translate-y-0 gap-0 overflow-hidden rounded-lg p-0 shadow-2xl"
        showClose={false}
      >
        <DialogHeader className="sr-only">
          <DialogTitle>Switch branch</DialogTitle>
          <DialogDescription>
            Select a local branch for this workspace repository.
          </DialogDescription>
        </DialogHeader>
        <div className="border-line/50 border-b p-1.5">
          <TextField
            autoFocus
            className="bg-surface min-h-8 rounded-md px-2 text-xs"
            label="Switch branch"
            labelHidden
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={handleInputKeyDown}
            placeholder="Search branches"
            size="small"
            value={query}
          />
        </div>
        <div className="grid max-h-[28rem] overflow-auto p-1.5">
          {isLoading ? (
            <p className="text-muted px-2 py-2 text-xs">Loading branches...</p>
          ) : null}
          {!isLoading && filteredBranches.length === 0 ? (
            <p className="text-muted px-2 py-2 text-xs">No branches found</p>
          ) : null}
          {filteredBranches.map((branch, index) => {
            const isCurrent = branch.current || branch.name === currentBranch;
            return (
              <BranchSwitchRow
                active={index === activeIndex}
                branch={branch}
                isCurrent={isCurrent}
                isSwitching={isSwitching}
                disabled={isCurrent || isSwitching}
                key={branch.name}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => void handleSwitch(branch.name)}
              />
            );
          })}
        </div>
        {error ? (
          <p className="text-warning border-line/50 border-t px-3 py-2 text-xs">
            {error}
          </p>
        ) : null}
      </DialogContent>
    </DialogRoot>
  );
}

function BranchSwitchRow({
  active,
  branch,
  disabled,
  isCurrent,
  isSwitching,
  onClick,
  onMouseEnter,
}: {
  active: boolean;
  branch: GitBranch;
  disabled: boolean;
  isCurrent: boolean;
  isSwitching: boolean;
  onClick: () => void;
  onMouseEnter: () => void;
}) {
  return (
    <button
      className={cn(
        "grid min-h-7 cursor-pointer grid-cols-[1.5rem_minmax(0,1fr)_auto] items-center gap-1.5 rounded-md px-1.5 text-left text-xs transition disabled:cursor-default disabled:opacity-80",
        active ? "bg-accent-soft text-ink" : "hover:bg-hover text-ink",
      )}
      disabled={disabled}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      type="button"
    >
      <span className="text-accent grid place-items-center">
        {isCurrent ? (
          <Check aria-hidden="true" className="size-3.5" />
        ) : (
          <GitBranchIcon aria-hidden="true" className="size-3.5" />
        )}
      </span>
      <span className="min-w-0 truncate font-mono text-xs font-semibold">
        {branch.name}
      </span>
      <span className="text-muted truncate text-[11px]">
        {isCurrent ? "current" : isSwitching ? "switching" : "branch"}
      </span>
    </button>
  );
}
