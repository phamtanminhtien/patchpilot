import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import { Check, Edit3, Loader2, Plus, Square, Terminal, X } from "lucide-react";
import { type FormEvent, useEffect, useRef, useState } from "react";

import { useAppearance } from "@/app/appearance";
import { type TerminalSession, terminalSocketUrl } from "@/shared/api";
import { Button, cn, StatusPill } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { MainEmptyState } from "../components/main-empty-state";

export function TerminalPanel({
  activeSession,
  activeSessionId,
  closeError,
  createError,
  isClosing,
  isCreating,
  isLoading,
  isRenaming,
  onClose,
  onCreate,
  onRename,
  onSelect,
  renameError,
  sessions,
  workspaceId,
}: {
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
  workspaceId: string;
}) {
  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
      <TerminalToolbar
        activeSession={activeSession}
        activeSessionId={activeSessionId}
        closeError={closeError}
        createError={createError}
        isClosing={isClosing}
        isCreating={isCreating}
        isLoading={isLoading}
        isRenaming={isRenaming}
        onClose={onClose}
        onCreate={onCreate}
        onRename={onRename}
        onSelect={onSelect}
        renameError={renameError}
        sessions={sessions}
      />

      {activeSession ? (
        <TerminalSurface session={activeSession} workspaceId={workspaceId} />
      ) : (
        <TerminalEmptyState isCreating={isCreating} onCreate={onCreate} />
      )}
    </div>
  );
}

function TerminalToolbar({
  activeSession,
  activeSessionId,
  closeError,
  createError,
  isClosing,
  isCreating,
  isLoading,
  isRenaming,
  onClose,
  onCreate,
  onRename,
  onSelect,
  renameError,
  sessions,
}: {
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
}) {
  const [editingSessionId, setEditingSessionId] = useState("");
  const [titleDraft, setTitleDraft] = useState("");
  const isEditingActive = activeSession?.id === editingSessionId;
  const hasError = Boolean(createError || renameError || closeError);

  function beginRename(session: TerminalSession) {
    setEditingSessionId(session.id);
    setTitleDraft(session.title);
  }

  function handleRename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeSession) {
      return;
    }
    onRename(activeSession.id, titleDraft);
  }

  return (
    <div className="bg-surface grid gap-1.5 px-2 py-0.5">
      <div className="flex min-w-0 items-center gap-1.5">
        <div
          aria-label="Terminal sessions"
          className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
          role="tablist"
        >
          {isLoading && sessions.length === 0 ? (
            <span className="text-muted inline-flex min-h-8 items-center gap-2 px-2 text-xs">
              <Loader2 aria-hidden="true" className="size-3.5 animate-spin" />
              Loading terminals
            </span>
          ) : null}
          {sessions.map((session) => (
            <TerminalTab
              isActive={activeSessionId === session.id}
              key={session.id}
              onSelect={onSelect}
              session={session}
            />
          ))}
        </div>

        {activeSession ? <StatusPill status={activeSession.status} /> : null}
        {activeSession ? (
          <Button
            aria-label="Rename terminal"
            icon={isRenaming ? <Loader2 className="animate-spin" /> : <Edit3 />}
            onClick={() => beginRename(activeSession)}
            size="icon"
            type="button"
            variant="action"
          />
        ) : null}
        <Button
          aria-label="New terminal"
          disabled={isCreating}
          icon={isCreating ? <Loader2 className="animate-spin" /> : <Plus />}
          onClick={onCreate}
          size="icon"
          type="button"
          variant="action"
        />
        <Button
          aria-label="Close terminal"
          disabled={isClosing || activeSession?.status !== "open"}
          icon={isClosing ? <Loader2 className="animate-spin" /> : <Square />}
          onClick={() => {
            if (activeSession) {
              onClose(activeSession.id);
            }
          }}
          size="icon"
          type="button"
          variant="action"
        />
      </div>

      {isEditingActive && activeSession ? (
        <form
          className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-1.5"
          onSubmit={handleRename}
        >
          <label className="sr-only" htmlFor="terminal-title">
            Session title
          </label>
          <input
            className="bg-panel text-ink min-h-8 min-w-0 rounded-xl px-2.5 text-xs outline-none focus-visible:shadow-[inset_0_0_0_1px_var(--pp-color-focus)]"
            id="terminal-title"
            onChange={(event) => setTitleDraft(event.target.value)}
            value={titleDraft}
          />
          <Button
            aria-label="Rename"
            disabled={
              isRenaming ||
              titleDraft.trim().length === 0 ||
              titleDraft === activeSession.title
            }
            icon={isRenaming ? <Loader2 className="animate-spin" /> : <Check />}
            size="icon"
            type="submit"
            variant="primary"
          />
          <Button
            aria-label="Cancel rename"
            icon={<X />}
            onClick={() => setEditingSessionId("")}
            size="icon"
            type="button"
            variant="action"
          />
        </form>
      ) : null}

      {hasError ? (
        <div>
          {createError ? <ErrorState message={createError} /> : null}
          {renameError ? <ErrorState message={renameError} /> : null}
          {closeError ? <ErrorState message={closeError} /> : null}
        </div>
      ) : null}
    </div>
  );
}

function TerminalTab({
  isActive,
  onSelect,
  session,
}: {
  isActive: boolean;
  onSelect: (sessionId: string) => void;
  session: TerminalSession;
}) {
  return (
    <button
      aria-selected={isActive}
      className={cn(
        "text-muted hover:bg-hover hover:text-ink grid min-h-8 max-w-44 shrink-0 cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-1.5 rounded-xl px-2 text-left text-xs transition",
        isActive ? "bg-panel text-ink" : undefined,
      )}
      onClick={() => onSelect(session.id)}
      role="tab"
      title={`${session.title} · ${session.status} · ${session.rows}x${session.cols}`}
      type="button"
    >
      <Terminal aria-hidden="true" className="size-3.5 shrink-0" />
      <span className="truncate font-medium">{session.title}</span>
    </button>
  );
}

function TerminalSurface({
  session,
  workspaceId,
}: {
  session: TerminalSession;
  workspaceId: string;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const { terminalFontFamily } = useAppearance();

  useEffect(() => {
    const container = containerRef.current;
    if (!container || session.status !== "open") {
      return;
    }
    const styles = getComputedStyle(document.documentElement);
    const terminal = new XTerm({
      cursorBlink: true,
      fontFamily: terminalFontFamily,
      fontSize: 12,
      scrollback: 1000,
      theme: {
        background: styles.getPropertyValue("--pp-bg-terminal").trim(),
        cursor: styles.getPropertyValue("--pp-color-focus").trim(),
        foreground: styles.getPropertyValue("--pp-color-terminal-ink").trim(),
        selectionBackground: styles
          .getPropertyValue("--pp-bg-accent-soft")
          .trim(),
      },
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(container);
    fitAddon.fit();

    const socket = new WebSocket(terminalSocketUrl(workspaceId, session.id));
    socket.addEventListener("message", (event: MessageEvent<string>) => {
      const message = JSON.parse(event.data) as {
        data?: string;
        message?: string;
        type: string;
      };
      if (message.type === "output" && message.data) {
        terminal.write(message.data);
      }
      if (message.type === "closed") {
        terminal.writeln("\r\n[terminal closed]");
      }
      if (message.type === "error" && message.message) {
        terminal.writeln(`\r\n[${message.message}]`);
      }
    });
    const inputDisposable = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "input", data }));
      }
    });
    const sendResize = () => {
      fitAddon.fit();
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(
          JSON.stringify({
            type: "resize",
            cols: terminal.cols,
            rows: terminal.rows,
          }),
        );
      }
    };
    socket.addEventListener("open", sendResize);
    const resizeObserver = new ResizeObserver(sendResize);
    resizeObserver.observe(container);

    return () => {
      resizeObserver.disconnect();
      inputDisposable.dispose();
      socket.close();
      terminal.dispose();
    };
  }, [session.id, session.status, terminalFontFamily, workspaceId]);

  if (session.status !== "open") {
    return (
      <MainEmptyState
        icon={<Terminal aria-hidden="true" className="size-6" />}
        message="This terminal session is closed."
        title={session.title}
      />
    );
  }

  return (
    <div className="bg-terminal min-h-0 overflow-hidden p-1.5">
      <div
        aria-label={`Terminal ${session.title}`}
        className="h-full min-h-0 overflow-hidden"
        ref={containerRef}
      />
    </div>
  );
}

function TerminalEmptyState({
  isCreating,
  onCreate,
}: {
  isCreating: boolean;
  onCreate: () => void;
}) {
  return (
    <div className="bg-panel grid min-h-0 place-items-center overflow-hidden p-3">
      <div className="grid max-w-sm justify-items-center gap-2 text-center">
        <Terminal aria-hidden="true" className="text-muted size-5" />
        <div className="grid gap-1">
          <p className="text-ink text-sm font-semibold">No terminal session</p>
          <p className="text-muted text-xs">
            Create a shell at the workspace root.
          </p>
        </div>
        <Button
          disabled={isCreating}
          icon={isCreating ? <Loader2 className="animate-spin" /> : <Plus />}
          onClick={onCreate}
          size="small"
          type="button"
          variant="action"
        >
          New terminal
        </Button>
      </div>
    </div>
  );
}
