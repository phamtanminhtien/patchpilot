import { Bot } from "lucide-react";
import { Link } from "react-router";

import { Button, cn } from "@/shared/ui";

import type { WorkspacePanel } from "../workspace-panels";
import { workspacePanels } from "../workspace-panels";

export function ActivityRail({
  activePanel,
  onPanelChange,
  workspaceId,
}: {
  activePanel: WorkspacePanel;
  onPanelChange: (panel: WorkspacePanel) => void;
  workspaceId: string;
}) {
  return (
    <nav
      aria-label="Workspace tools"
      className="bg-panel flex min-w-0 gap-1 overflow-x-auto px-1.5 py-1.5 shadow-sm lg:min-h-0 lg:flex-col lg:items-center lg:overflow-visible"
    >
      {workspacePanels.map((item) => (
        <button
          aria-label={item.label}
          aria-pressed={activePanel === item.value}
          className={cn(
            "text-muted hover:bg-hover hover:text-ink grid aspect-square min-w-14 cursor-pointer place-items-center gap-0.5 rounded-md px-2 text-xs font-medium transition lg:size-10 lg:min-w-0",
            activePanel === item.value
              ? "bg-accent-soft text-accent shadow-sm"
              : undefined,
          )}
          key={item.value}
          onClick={() => onPanelChange(item.value)}
          title={item.label}
          type="button"
        >
          <item.icon aria-hidden="true" className="size-4" />
          <span className="truncate lg:sr-only">{item.label}</span>
        </button>
      ))}

      <Button
        asChild
        className="aspect-square min-w-14 px-2 text-xs lg:mt-auto lg:size-10 lg:min-w-0 lg:px-0"
        size="small"
        variant="ghost"
      >
        <Link
          aria-label="Open Vibe Mode"
          title="Open Vibe Mode"
          to={`/vibe?workspaceId=${encodeURIComponent(workspaceId)}`}
        >
          <Bot aria-hidden="true" className="size-4" />
          <span className="truncate lg:sr-only">Vibe</span>
        </Link>
      </Button>
    </nav>
  );
}
