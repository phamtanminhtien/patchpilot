import { useQuery } from "@tanstack/react-query";
import type { LucideIcon } from "lucide-react";
import {
  FileText,
  FolderGit2,
  GitBranch,
  MonitorUp,
  RefreshCw,
  Search,
} from "lucide-react";
import {
  Fragment,
  type KeyboardEvent as ReactKeyboardEvent,
  useEffect,
  useMemo,
  useState,
} from "react";

import { apiErrorMessage, listFileIndex } from "@/shared/api";
import { FileIcon } from "@/shared/file-icons";
import {
  cn,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  TextField,
} from "@/shared/ui";

import type { WorkspacePanel } from "../workspace-panels";

export interface CommandPaletteItem {
  disabled?: boolean;
  icon?: LucideIcon;
  id: string;
  keywords: string[];
  run: () => void;
  subtitle: string;
  title: string;
}

export function CommandPalette({
  onOpenFile,
  onPanelChange,
  onRefreshFiles,
  selectedPath,
  workspaceId,
}: {
  onOpenFile: (path: string) => void;
  onPanelChange: (panel: WorkspacePanel) => void;
  onRefreshFiles: () => void;
  selectedPath: string;
  workspaceId: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const isCommandMode = query.startsWith(">");
  const effectiveQuery = isCommandMode ? query.slice(1).trim() : query.trim();

  useEffect(() => {
    function handleKeyDown(event: globalThis.KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "p") {
        event.preventDefault();
        setActiveIndex(0);
        setOpen(true);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const fileQuery = useQuery({
    enabled: open && !isCommandMode && workspaceId.length > 0,
    queryFn: () =>
      listFileIndex(workspaceId, {
        kind: "file",
        limit: 50,
        q: effectiveQuery,
      }),
    queryKey: ["workspace-command-palette-files", workspaceId, effectiveQuery],
  });

  const commandItems = useMemo<CommandPaletteItem[]>(() => {
    const items: CommandPaletteItem[] = [
      panelCommand("Files", "Open file tree", FolderGit2, () =>
        onPanelChange("files"),
      ),
      panelCommand("Search", "Open content search", Search, () =>
        onPanelChange("search"),
      ),
      panelCommand("Git", "Open working tree", GitBranch, () =>
        onPanelChange("git"),
      ),
      panelCommand("Preview", "Open port preview", MonitorUp, () =>
        onPanelChange("preview"),
      ),
      {
        icon: RefreshCw,
        id: "refresh-files",
        keywords: ["refresh", "index", "files"],
        run: onRefreshFiles,
        subtitle: "Workspace index",
        title: "Refresh file index",
      },
    ];
    if (selectedPath.length > 0) {
      items.push({
        icon: FileText,
        id: "open-selected-file",
        keywords: ["open", "selected", selectedPath],
        run: () => onOpenFile(selectedPath),
        subtitle: selectedPath,
        title: "Open selected file",
      });
    }
    return items;
  }, [onOpenFile, onPanelChange, onRefreshFiles, selectedPath]);

  const items = isCommandMode
    ? filterCommandItems(commandItems, effectiveQuery)
    : (fileQuery.data?.entries ?? []).map<CommandPaletteItem>((entry) => ({
        id: `file:${entry.path}`,
        keywords: [entry.name ?? "", entry.path, entry.extension ?? ""],
        run: () => onOpenFile(entry.path),
        subtitle: entry.dir ?? "",
        title: entry.name || entry.path,
      }));

  const activeItem = items[activeIndex];

  function runItem(item: CommandPaletteItem | undefined) {
    if (!item || item.disabled) {
      return;
    }
    item.run();
    setOpen(false);
  }

  function handleInputKeyDown(event: ReactKeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) =>
        Math.min(index + 1, Math.max(items.length - 1, 0)),
      );
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(index - 1, 0));
    }
    if (event.key === "Enter") {
      event.preventDefault();
      runItem(activeItem);
    }
  }

  return (
    <DialogRoot
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        setActiveIndex(0);
        if (!nextOpen) {
          setQuery("");
        }
      }}
      open={open}
    >
      <DialogContent
        className="top-8 max-h-[min(34rem,calc(100vh-4rem))] w-[calc(100vw-2rem)] max-w-3xl translate-y-0 gap-0 overflow-hidden rounded-lg p-0 shadow-2xl"
        showClose={false}
      >
        <DialogHeader className="sr-only">
          <DialogTitle>Command Palette</DialogTitle>
          <DialogDescription>Search files or commands.</DialogDescription>
        </DialogHeader>
        <div className="border-line/50 border-b p-1.5">
          <TextField
            autoFocus
            className="bg-surface min-h-8 rounded-md px-2 text-xs"
            label="Command Palette"
            labelHidden
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={handleInputKeyDown}
            placeholder="Search files"
            size="small"
            value={query}
          />
        </div>
        <div className="grid max-h-[28rem] overflow-auto p-1.5">
          {!isCommandMode && fileQuery.error ? (
            <p className="text-warning px-2 py-2 text-xs">
              {apiErrorMessage(fileQuery.error)}
            </p>
          ) : null}
          {!fileQuery.error && items.length === 0 ? (
            <p className="text-muted px-2 py-2 text-xs">No results</p>
          ) : null}
          {items.map((item, index) => (
            <CommandPaletteRow
              active={index === activeIndex}
              item={item}
              key={item.id}
              mode={isCommandMode ? "command" : "file"}
              onMouseEnter={() => setActiveIndex(index)}
              onRun={() => runItem(item)}
              query={effectiveQuery}
            />
          ))}
        </div>
      </DialogContent>
    </DialogRoot>
  );
}

function CommandPaletteRow({
  active,
  item,
  mode,
  onMouseEnter,
  onRun,
  query,
}: {
  active: boolean;
  item: CommandPaletteItem;
  mode: "command" | "file";
  onMouseEnter: () => void;
  onRun: () => void;
  query: string;
}) {
  const Icon = item.icon;
  const itemPath = item.subtitle
    ? `${item.subtitle}/${item.title}`
    : item.title;
  return (
    <button
      className={cn(
        "grid min-h-7 cursor-pointer grid-cols-[1.5rem_minmax(0,1fr)_auto] items-center gap-1.5 rounded-md px-1.5 text-left text-xs transition",
        active ? "bg-accent-soft text-ink" : "hover:bg-hover text-ink",
      )}
      disabled={item.disabled}
      onClick={onRun}
      onMouseEnter={onMouseEnter}
      type="button"
    >
      <span className="text-accent grid place-items-center">
        {mode === "file" ? (
          <FileIcon className="size-3.5" path={itemPath} />
        ) : Icon ? (
          <Icon aria-hidden="true" className="size-3.5" />
        ) : null}
      </span>
      <span className="flex min-w-0 items-baseline gap-2">
        <span className="truncate text-xs font-semibold">
          {highlightFuzzyMatch(item.title, query)}
        </span>
        {item.subtitle ? (
          <span className="text-muted min-w-0 truncate text-xs font-medium">
            {highlightFuzzyMatch(item.subtitle, query)}
          </span>
        ) : null}
      </span>
      <span className="text-muted flex shrink-0 items-center gap-2 text-[11px] font-semibold">
        {active ? <span>{mode === "file" ? "open" : "run"}</span> : null}
        <span className="hidden sm:inline">
          {mode === "file" ? "file" : "command"}
        </span>
      </span>
    </button>
  );
}

function panelCommand(
  title: string,
  subtitle: string,
  icon: LucideIcon,
  run: () => void,
): CommandPaletteItem {
  return {
    icon,
    id: `panel:${title.toLowerCase()}`,
    keywords: [title, subtitle, "panel"],
    run,
    subtitle,
    title,
  };
}

function filterCommandItems(items: CommandPaletteItem[], query: string) {
  const normalized = query.toLowerCase();
  if (normalized.length === 0) {
    return items;
  }
  return items
    .map((item) => ({ item, score: commandMatchScore(item, normalized) }))
    .filter((item) => item.score !== null)
    .sort((left, right) => {
      if (left.score !== right.score) {
        return (left.score ?? 0) - (right.score ?? 0);
      }
      return left.item.title.localeCompare(right.item.title);
    })
    .map(({ item }) => item);
}

function commandMatchScore(item: CommandPaletteItem, query: string) {
  const fields = [item.title, item.subtitle, ...item.keywords].filter(Boolean);
  let best: number | null = null;
  for (const field of fields) {
    const score = textMatchScore(field.toLowerCase(), query);
    if (score !== null && (best === null || score < best)) {
      best = score;
    }
  }
  return best;
}

function textMatchScore(value: string, query: string) {
  if (value === query) {
    return 0;
  }
  if (value.startsWith(query)) {
    return 10;
  }
  const containsIndex = value.indexOf(query);
  if (containsIndex !== -1) {
    return 20 + containsIndex;
  }
  const positions = fuzzyMatchPositions(value, query);
  if (!positions) {
    return null;
  }
  const gaps = positions.slice(1).reduce((total, position, index) => {
    return total + position - (positions[index] ?? position) - 1;
  }, 0);
  return 80 + (positions[0] ?? 0) * 2 + gaps + value.length - query.length;
}

function highlightFuzzyMatch(value: string, query: string) {
  if (query.trim().length === 0) {
    return value;
  }
  const positions = fuzzyMatchPositions(
    value.toLowerCase(),
    query.toLowerCase(),
  );
  if (!positions) {
    return value;
  }
  const highlighted = new Set(positions);
  return Array.from(value).map((char, index) => (
    <Fragment key={`${char}-${index}`}>
      {highlighted.has(index) ? (
        <span className="text-accent font-bold">{char}</span>
      ) : (
        char
      )}
    </Fragment>
  ));
}

function fuzzyMatchPositions(value: string, query: string) {
  if (query.length === 0) {
    return [];
  }
  const positions: number[] = [];
  let queryIndex = 0;
  for (const [index, char] of Array.from(value).entries()) {
    if (char !== query[queryIndex]) {
      continue;
    }
    positions.push(index);
    queryIndex += 1;
    if (queryIndex === query.length) {
      return positions;
    }
  }
  return null;
}
