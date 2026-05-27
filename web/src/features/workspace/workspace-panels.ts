import { Files, GitBranch, type LucideIcon, MonitorUp } from "lucide-react";

export interface WorkspacePanelDefinition {
  description: string;
  icon: LucideIcon;
  label: string;
  value: WorkspacePanel;
}

export const workspacePanelValues = ["files", "git", "preview"] as const;

export type WorkspacePanel = (typeof workspacePanelValues)[number];

export const workspacePanels = [
  {
    description: "Browse files and inspect small text content.",
    icon: Files,
    label: "Files",
    value: "files",
  },
  {
    description: "Review working tree status and selected diffs.",
    icon: GitBranch,
    label: "Git",
    value: "git",
  },
  {
    description: "Prepare same-host preview controls for detected ports.",
    icon: MonitorUp,
    label: "Preview",
    value: "preview",
  },
] as const satisfies readonly WorkspacePanelDefinition[];

export function panelShortDescription(panel: WorkspacePanel) {
  switch (panel) {
    case "files":
      return "File tree";
    case "git":
      return "Working tree";
    case "preview":
      return "Port preview";
  }
}
