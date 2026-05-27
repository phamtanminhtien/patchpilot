import type { TerminalSession } from "@/shared/api";

import { TerminalPanel } from "../panels/terminal-panel";

export function WorkspaceBottomPanel({
  terminal,
  workspaceId,
}: {
  terminal: {
    activeSession: TerminalSession | null;
    activeSessionId: string;
    closeError?: string;
    createError?: string;
    isClosing: boolean;
    isCreating: boolean;
    isLoading: boolean;
    isRenaming: boolean;
    onClose: (sessionId: string) => void;
    onCreate: () => void;
    onRename: (sessionId: string, title: string) => void;
    onSelect: (sessionId: string) => void;
    renameError?: string;
    sessions: TerminalSession[];
  };
  workspaceId: string;
}) {
  return (
    <section className="border-line/45 bg-panel min-h-0 overflow-hidden border-t">
      <TerminalPanel
        activeSession={terminal.activeSession}
        activeSessionId={terminal.activeSessionId}
        closeError={terminal.closeError}
        createError={terminal.createError}
        isClosing={terminal.isClosing}
        isCreating={terminal.isCreating}
        isLoading={terminal.isLoading}
        isRenaming={terminal.isRenaming}
        onClose={terminal.onClose}
        onCreate={terminal.onCreate}
        onRename={terminal.onRename}
        onSelect={terminal.onSelect}
        renameError={terminal.renameError}
        sessions={terminal.sessions}
        workspaceId={workspaceId}
      />
    </section>
  );
}
