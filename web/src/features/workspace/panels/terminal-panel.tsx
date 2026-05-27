import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import { Loader2, Plus, Square, Terminal, X } from "lucide-react";
import { type FormEvent, useEffect, useRef, useState } from "react";

import { type TerminalSession, terminalSocketUrl } from "@/shared/api";
import { Button, cn, StatusPill, TextField } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
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
    <div className="grid h-full min-h-0 grid-cols-[minmax(0,1fr)_8.5rem] overflow-hidden sm:grid-cols-[minmax(0,1fr)_12rem] lg:grid-cols-[minmax(0,1fr)_14rem]">
      <section className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
        {activeSession ? (
          <>
            <TerminalSessionHeader
              closeError={closeError}
              createError={createError}
              isClosing={isClosing}
              isRenaming={isRenaming}
              key={activeSession.id}
              onClose={onClose}
              onRename={onRename}
              renameError={renameError}
              session={activeSession}
            />
            <TerminalSurface
              session={activeSession}
              workspaceId={workspaceId}
            />
          </>
        ) : (
          <TerminalEmptyState isCreating={isCreating} onCreate={onCreate} />
        )}
      </section>

      <aside className="border-line bg-panel grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden border-l">
        <div className="border-line flex min-h-10 items-center justify-between gap-1.5 border-b px-2 py-1.5">
          <p className="text-ink truncate text-xs font-semibold">Terminal</p>
          <Button
            aria-label="New terminal"
            disabled={isCreating}
            icon={isCreating ? <Loader2 className="animate-spin" /> : <Plus />}
            onClick={onCreate}
            size="small"
            type="button"
            variant="action"
          />
        </div>
        <TerminalSessionList
          activeSessionId={activeSessionId}
          isClosing={isClosing}
          isLoading={isLoading}
          onClose={onClose}
          onSelect={onSelect}
          sessions={sessions}
        />
      </aside>
    </div>
  );
}

function TerminalSessionHeader({
  closeError,
  createError,
  isClosing,
  isRenaming,
  onClose,
  onRename,
  renameError,
  session,
}: {
  closeError?: string;
  createError?: string;
  isClosing: boolean;
  isRenaming: boolean;
  onClose: (sessionId: string) => void;
  onRename: (sessionId: string, title: string) => void;
  renameError?: string;
  session: TerminalSession;
}) {
  const [titleDraft, setTitleDraft] = useState(session.title);

  function handleRename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onRename(session.id, titleDraft);
  }

  return (
    <div className="bg-panel border-line grid gap-2 border-b p-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
      <form className="grid min-w-0 gap-1.5" onSubmit={handleRename}>
        <TextField
          className="bg-hover"
          id="terminal-title"
          label="Session title"
          labelClassName="text-muted font-semibold"
          onChange={(event) => setTitleDraft(event.target.value)}
          size="compact"
          value={titleDraft}
        />
      </form>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        <StatusPill status={session.status} />
        <Button
          disabled={
            isRenaming ||
            titleDraft.trim().length === 0 ||
            titleDraft === session.title
          }
          icon={isRenaming ? <Loader2 className="animate-spin" /> : undefined}
          onClick={() => onRename(session.id, titleDraft)}
          size="small"
          type="button"
          variant="secondary"
        >
          Rename
        </Button>
        <Button
          disabled={isClosing || session.status !== "open"}
          icon={isClosing ? <Loader2 className="animate-spin" /> : <Square />}
          onClick={() => onClose(session.id)}
          size="small"
          type="button"
          variant="secondary"
        >
          Close
        </Button>
      </div>
      <div className="sm:col-span-2">
        {createError ? <ErrorState message={createError} /> : null}
        {renameError ? <ErrorState message={renameError} /> : null}
        {closeError ? <ErrorState message={closeError} /> : null}
      </div>
    </div>
  );
}

function TerminalSessionList({
  activeSessionId,
  isClosing,
  isLoading,
  onClose,
  onSelect,
  sessions,
}: {
  activeSessionId: string;
  isClosing: boolean;
  isLoading: boolean;
  onClose: (sessionId: string) => void;
  onSelect: (sessionId: string) => void;
  sessions: TerminalSession[];
}) {
  if (isLoading && sessions.length === 0) {
    return <LoadingState label="Loading terminal sessions" />;
  }
  if (sessions.length === 0) {
    return (
      <div className="p-3">
        <p className="text-muted text-xs">No terminal sessions.</p>
      </div>
    );
  }

  return (
    <div className="min-h-0 overflow-auto">
      {sessions.map((session) => (
        <div
          className={cn(
            "border-line hover:bg-hover grid min-h-11 grid-cols-[minmax(0,1fr)_auto] items-center gap-1.5 border-b px-2 py-1.5 transition",
            activeSessionId === session.id
              ? "bg-hover text-ink shadow-[inset_-3px_0_0_var(--pp-color-focus)]"
              : undefined,
          )}
          key={session.id}
        >
          <button
            className="min-w-0 text-left"
            onClick={() => onSelect(session.id)}
            type="button"
          >
            <span className="text-ink block truncate text-xs font-semibold">
              {session.title}
            </span>
            <span className="text-muted block truncate text-xs">
              {session.status} · {session.rows}x{session.cols}
            </span>
          </button>
          <Button
            aria-label={`Close terminal ${session.title}`}
            disabled={isClosing || session.status !== "open"}
            icon={<X />}
            onClick={() => onClose(session.id)}
            size="small"
            type="button"
            variant="secondary"
          />
        </div>
      ))}
    </div>
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

  useEffect(() => {
    const container = containerRef.current;
    if (!container || session.status !== "open") {
      return;
    }
    const styles = getComputedStyle(document.documentElement);
    const terminal = new XTerm({
      cursorBlink: true,
      fontFamily:
        "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
      fontSize: 12,
      scrollback: 1000,
      theme: {
        background: styles.getPropertyValue("--pp-bg-terminal").trim(),
        cursor: styles.getPropertyValue("--pp-color-focus").trim(),
        foreground: styles.getPropertyValue("--pp-color-ink").trim(),
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
  }, [session.id, session.status, workspaceId]);

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
    <div className="bg-canvas grid min-h-0 place-items-center overflow-hidden p-3">
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
