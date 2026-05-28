import { ChevronDown, GitBranch, Minus, Plus } from "lucide-react";
import { useState } from "react";
import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import css from "react-syntax-highlighter/dist/esm/languages/prism/css";
import diffLanguage from "react-syntax-highlighter/dist/esm/languages/prism/diff";
import docker from "react-syntax-highlighter/dist/esm/languages/prism/docker";
import go from "react-syntax-highlighter/dist/esm/languages/prism/go";
import javascript from "react-syntax-highlighter/dist/esm/languages/prism/javascript";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import jsx from "react-syntax-highlighter/dist/esm/languages/prism/jsx";
import markdown from "react-syntax-highlighter/dist/esm/languages/prism/markdown";
import markup from "react-syntax-highlighter/dist/esm/languages/prism/markup";
import tsx from "react-syntax-highlighter/dist/esm/languages/prism/tsx";
import typescript from "react-syntax-highlighter/dist/esm/languages/prism/typescript";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";

import { Button, cn } from "@/shared/ui";

import {
  parseUnifiedDiff,
  type UnifiedDiffFile,
  type UnifiedDiffHunk,
  type UnifiedDiffLine,
} from "./unified-diff";

SyntaxHighlighter.registerLanguage("css", css);
SyntaxHighlighter.registerLanguage("diff", diffLanguage);
SyntaxHighlighter.registerLanguage("docker", docker);
SyntaxHighlighter.registerLanguage("go", go);
SyntaxHighlighter.registerLanguage("javascript", javascript);
SyntaxHighlighter.registerLanguage("json", json);
SyntaxHighlighter.registerLanguage("jsx", jsx);
SyntaxHighlighter.registerLanguage("markdown", markdown);
SyntaxHighlighter.registerLanguage("markup", markup);
SyntaxHighlighter.registerLanguage("tsx", tsx);
SyntaxHighlighter.registerLanguage("typescript", typescript);
SyntaxHighlighter.registerLanguage("yaml", yaml);

export function DiffViewer({
  actionLabel,
  className,
  diff,
  isActionPending = false,
  onFileAction,
  onHunkAction,
  selectedPath,
  wrapLines = false,
}: {
  actionLabel?: string;
  className?: string;
  diff: string;
  isActionPending?: boolean;
  onFileAction?: (file: UnifiedDiffFile) => void;
  onHunkAction?: (hunk: UnifiedDiffHunk, file: UnifiedDiffFile) => void;
  selectedPath?: string;
  wrapLines?: boolean;
}) {
  const summary = parseUnifiedDiff(diff);
  const files = selectedPath
    ? summary.files.filter((file) => file.path === selectedPath)
    : summary.files;
  const [openFileIds, setOpenFileIds] = useState<Set<string>>(() => new Set());

  const toggleFile = (fileId: string) => {
    setOpenFileIds((current) => {
      const next = new Set(current);
      if (next.has(fileId)) {
        next.delete(fileId);
      } else {
        next.add(fileId);
      }
      return next;
    });
  };

  if (summary.files.length === 0) {
    return (
      <pre
        className={cn(
          "text-ink overflow-auto font-mono text-xs whitespace-pre-wrap",
          className,
        )}
      >
        {diff}
      </pre>
    );
  }

  return (
    <div
      className={cn(
        "pp-diff-viewer grid min-h-0 grid-rows-[auto_minmax(0,1fr)]",
        className,
      )}
    >
      <div className="border-line/35 bg-surface flex min-h-9 min-w-0 items-center justify-between gap-2 border-b px-3">
        <span className="text-ink min-w-0 truncate text-xs font-semibold">
          {summary.files.length} {summary.files.length === 1 ? "file" : "files"}
        </span>
        <DiffStats
          additions={summary.additions}
          deletions={summary.deletions}
        />
      </div>
      <div className="workspace-main-scroll min-h-0 overflow-auto">
        {files.map((file) => {
          const isOpen = openFileIds.has(file.id);

          return (
            <section className="border-line/35 border-b" key={file.id}>
              <div className="bg-panel/60 sticky top-0 z-10 grid min-h-9 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 px-3">
                <button
                  aria-expanded={isOpen}
                  className="grid min-w-0 cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-1 text-left"
                  onClick={() => toggleFile(file.id)}
                  type="button"
                >
                  <ChevronDown
                    aria-hidden="true"
                    className={cn(
                      "text-muted size-4 transition-transform",
                      isOpen ? undefined : "-rotate-90",
                    )}
                  />
                  <span className="min-w-0 truncate font-mono text-xs font-semibold">
                    <span className="text-ink">{file.path}</span>
                    {wrapLines && file.hunks.length === 1 ? (
                      <span className="text-muted ml-2 font-normal">
                        {file.hunks[0]?.header}
                      </span>
                    ) : null}
                  </span>
                </button>
                <DiffStats
                  additions={file.additions}
                  deletions={file.deletions}
                />
                {onFileAction && actionLabel ? (
                  <Button
                    className="min-h-7 px-2"
                    disabled={isActionPending}
                    icon={<GitBranch aria-hidden="true" className="size-3.5" />}
                    onClick={() => onFileAction(file)}
                    size="small"
                    type="button"
                    variant="secondary"
                  >
                    {actionLabel}
                  </Button>
                ) : null}
              </div>
              <div
                aria-hidden={!isOpen}
                className={cn(
                  "grid overflow-hidden transition-[grid-template-rows,opacity] duration-200 ease-out",
                  isOpen
                    ? "grid-rows-[1fr] opacity-100"
                    : "grid-rows-[0fr] opacity-0",
                )}
                data-diff-file-content="true"
              >
                <div className="min-h-0 overflow-hidden">
                  {file.hunks.map((hunk) => (
                    <DiffHunk
                      actionLabel={actionLabel}
                      file={file}
                      hunk={hunk}
                      isActionPending={isActionPending}
                      key={hunk.id}
                      onHunkAction={onHunkAction}
                      showHeader={!(wrapLines && file.hunks.length === 1)}
                      wrapLines={wrapLines}
                    />
                  ))}
                </div>
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}

function DiffHunk({
  actionLabel,
  file,
  hunk,
  isActionPending,
  onHunkAction,
  showHeader,
  wrapLines,
}: {
  actionLabel?: string;
  file: UnifiedDiffFile;
  hunk: UnifiedDiffHunk;
  isActionPending: boolean;
  onHunkAction?: (hunk: UnifiedDiffHunk, file: UnifiedDiffFile) => void;
  showHeader: boolean;
  wrapLines: boolean;
}) {
  return (
    <div className="grid min-w-0">
      {showHeader ? (
        <div
          className="border-line/25 bg-surface/75 grid min-h-8 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 border-y px-3"
          data-diff-hunk-header="true"
        >
          <span className="text-muted min-w-0 truncate font-mono text-xs">
            {hunk.header}
          </span>
          <DiffStats additions={hunk.additions} deletions={hunk.deletions} />
          {onHunkAction && actionLabel ? (
            <Button
              className="min-h-7 px-2"
              disabled={isActionPending}
              icon={<GitBranch aria-hidden="true" className="size-3.5" />}
              onClick={() => onHunkAction(hunk, file)}
              size="small"
              type="button"
              variant="ghost"
            >
              {actionLabel} hunk
            </Button>
          ) : null}
        </div>
      ) : null}
      <div className="font-mono text-xs leading-5">
        {hunk.lines.map((line, index) => (
          <DiffLine
            filePath={file.path}
            key={`${hunk.id}-${index}`}
            line={line}
            wrapLines={wrapLines}
          />
        ))}
      </div>
    </div>
  );
}

function DiffLine({
  filePath,
  line,
  wrapLines,
}: {
  filePath: string;
  line: UnifiedDiffLine;
  wrapLines: boolean;
}) {
  const marker = line.kind === "add" ? "+" : line.kind === "delete" ? "-" : "";
  const lineNumber = line.kind === "delete" ? line.oldLine : line.newLine;

  return (
    <div
      className={cn(
        "grid px-3",
        wrapLines
          ? "min-w-0 grid-cols-[3rem_1.25rem_minmax(0,1fr)]"
          : "min-w-max grid-cols-[4rem_1.5rem_minmax(28rem,1fr)]",
        line.kind === "add"
          ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
          : undefined,
        line.kind === "delete"
          ? "bg-rose-500/10 text-rose-700 dark:text-rose-300"
          : undefined,
        line.kind === "meta" ? "text-muted" : undefined,
      )}
    >
      <span className="text-muted pr-3 text-right select-none">
        {lineNumber ?? ""}
      </span>
      <span className="pr-2 select-none">{marker}</span>
      <span
        className={cn(
          "min-w-0",
          wrapLines ? "break-words whitespace-pre-wrap" : "whitespace-pre",
        )}
      >
        {line.kind === "meta" ? (
          line.raw
        ) : (
          <DiffCodeText
            path={filePath}
            text={line.text}
            wrapLines={wrapLines}
          />
        )}
      </span>
    </div>
  );
}

function DiffCodeText({
  path,
  text,
  wrapLines,
}: {
  path: string;
  text: string;
  wrapLines: boolean;
}) {
  const language = syntaxForPath(path);

  if (!language) {
    return text;
  }

  return (
    <SyntaxHighlighter
      CodeTag="span"
      PreTag="span"
      language={language}
      style={{}}
      useInlineStyles={false}
      wrapLongLines={wrapLines}
    >
      {text || " "}
    </SyntaxHighlighter>
  );
}

function syntaxForPath(path: string) {
  const basename = path.split("/").pop()?.toLowerCase() ?? "";
  const extension = basename.split(".").pop() ?? "";

  if (basename === "dockerfile") {
    return "docker";
  }

  switch (extension) {
    case "cjs":
    case "js":
    case "mjs":
      return "javascript";
    case "jsx":
      return "jsx";
    case "ts":
      return "typescript";
    case "tsx":
      return "tsx";
    case "json":
      return "json";
    case "html":
    case "htm":
      return "markup";
    case "css":
      return "css";
    case "md":
    case "mdx":
    case "markdown":
      return "markdown";
    case "go":
      return "go";
    case "yaml":
    case "yml":
      return "yaml";
    default:
      return undefined;
  }
}

export function DiffStats({
  additions,
  deletions,
}: {
  additions: number;
  deletions: number;
}) {
  return (
    <span className="flex shrink-0 items-center gap-2 text-xs font-semibold">
      <span className="flex items-center gap-0.5 text-emerald-600 dark:text-emerald-300">
        <Plus aria-hidden="true" className="size-3" />
        {additions}
      </span>
      <span className="flex items-center gap-0.5 text-rose-600 dark:text-rose-300">
        <Minus aria-hidden="true" className="size-3" />
        {deletions}
      </span>
    </span>
  );
}
