export interface WorkspaceGitStatusLike {
  code: string;
  label: string;
}

export function gitStatusBadgeCode(status: WorkspaceGitStatusLike) {
  if (status.label === "Untracked") {
    return "U";
  }

  return status.code.trim() || "--";
}

export function gitStatusBadgeTone(status: string) {
  switch (status) {
    case "Deleted":
    case "Conflict":
      return "bg-panel text-warning";
    case "Ignored":
      return "bg-hover text-muted";
    case "Added":
    case "Renamed":
    case "Copied":
    case "Untracked":
    case "Changed":
    case "Modified":
      return "bg-accent-soft text-accent";
    default:
      return "bg-hover text-muted";
  }
}

export function gitStatusTextTone(status: string) {
  switch (status) {
    case "Ignored":
      return "text-muted";
    case "Deleted":
    case "Conflict":
      return "text-warning";
    case "Added":
    case "Renamed":
    case "Copied":
    case "Untracked":
    case "Changed":
    case "Modified":
      return "text-accent";
    default:
      return "text-ink";
  }
}
