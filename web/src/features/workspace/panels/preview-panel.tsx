import { MonitorUp } from "lucide-react";

import { MainEmptyState } from "../components/main-empty-state";

export function PreviewPanel() {
  return (
    <MainEmptyState
      icon={<MonitorUp aria-hidden="true" className="size-6" />}
      message="Same-host preview controls will appear here after process and port APIs are implemented."
      title="Preview unavailable"
    />
  );
}
