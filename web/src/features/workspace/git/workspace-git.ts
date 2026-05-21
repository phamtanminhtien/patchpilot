export interface GitChange {
  code: string;
  displayPath: string;
  id: string;
  path: string;
  raw: string;
  status: string;
}

export function parseGitPorcelain(porcelain: string): GitChange[] {
  return porcelain
    .split(/\r?\n/)
    .filter((line) => line.length > 0)
    .map((line, index) => {
      const indexStatus = line[0] ?? " ";
      const worktreeStatus = line[1] ?? " ";
      const code = `${indexStatus}${worktreeStatus}`;
      const displayPath = line.slice(3).trim();
      const path = pathFromDisplay(displayPath);

      return {
        code,
        displayPath,
        id: `${index}-${code}-${path}`,
        path,
        raw: line,
        status: statusLabel(indexStatus, worktreeStatus),
      };
    });
}

export function stagedGitPaths(changes: GitChange[]) {
  return changes.filter(isStagedGitChange).map((change) => change.path);
}

export function unstagedGitPaths(changes: GitChange[]) {
  return changes
    .filter(
      (change) => isUnstagedGitChange(change) && isGitChangeStageable(change),
    )
    .map((change) => change.path);
}

export function visibleGitChanges(changes: GitChange[]) {
  return changes.filter((change) => !isIgnoredGitChange(change));
}

export function isGitChangeStageable(change: GitChange) {
  return change.status !== "Ignored";
}

export function isIgnoredGitChange(change: GitChange) {
  return change.status === "Ignored";
}

export function isStagedGitChange(change: GitChange) {
  const indexStatus = change.code[0] ?? " ";
  return indexStatus !== " " && indexStatus !== "?" && indexStatus !== "!";
}

export function isUnstagedGitChange(change: GitChange) {
  const worktreeStatus = change.code[1] ?? " ";
  return worktreeStatus !== " ";
}

function pathFromDisplay(displayPath: string) {
  const renameSeparator = " -> ";
  if (!displayPath.includes(renameSeparator)) {
    return displayPath;
  }
  return displayPath.split(renameSeparator).at(-1) ?? displayPath;
}

function statusLabel(indexStatus: string, worktreeStatus: string) {
  if (indexStatus === "?" && worktreeStatus === "?") {
    return "Untracked";
  }
  if (indexStatus === "!" && worktreeStatus === "!") {
    return "Ignored";
  }
  if (indexStatus === "U" || worktreeStatus === "U") {
    return "Conflict";
  }
  if (indexStatus === "R" || worktreeStatus === "R") {
    return "Renamed";
  }
  if (indexStatus === "C" || worktreeStatus === "C") {
    return "Copied";
  }
  if (indexStatus === "A" || worktreeStatus === "A") {
    return "Added";
  }
  if (indexStatus === "D" || worktreeStatus === "D") {
    return "Deleted";
  }
  if (indexStatus === "M" || worktreeStatus === "M") {
    return "Modified";
  }
  return "Changed";
}
