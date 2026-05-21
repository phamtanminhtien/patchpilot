import {
  Files,
  GitBranch,
  type LucideIcon,
  MonitorUp,
  Play,
} from "lucide-react";

export interface WorkspacePanelDefinition {
  description: string;
  icon: LucideIcon;
  label: string;
  value: WorkspacePanel;
}

export const workspacePanelValues = [
  "files",
  "git",
  "commands",
  "preview",
] as const;

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
    description: "Queue a direct command from the workspace root.",
    icon: Play,
    label: "Commands",
    value: "commands",
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
    case "commands":
      return "Queue status";
    case "preview":
      return "Port preview";
  }
}
