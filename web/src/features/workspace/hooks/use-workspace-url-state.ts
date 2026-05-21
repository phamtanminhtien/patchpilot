import { useQueryState } from "nuqs";

import { panelParser, pathParser, workspaceIdParser } from "@/shared/url";

export function useWorkspaceUrlState() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [panel, setPanel] = useQueryState("panel", panelParser);
  const [selectedPath, setSelectedPath] = useQueryState("path", pathParser);

  return {
    panel,
    selectedPath,
    setPanel,
    setSelectedPath,
    setWorkspaceId,
    workspaceId,
  };
}
