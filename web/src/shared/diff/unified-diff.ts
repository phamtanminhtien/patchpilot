export type DiffLineKind = "add" | "context" | "delete" | "meta";

export interface UnifiedDiffLine {
  kind: DiffLineKind;
  oldLine?: number;
  raw: string;
  text: string;
  newLine?: number;
}

export interface UnifiedDiffHunk {
  additions: number;
  deletions: number;
  header: string;
  id: string;
  lines: UnifiedDiffLine[];
  patch: string;
}

export interface UnifiedDiffFile {
  additions: number;
  deletions: number;
  headerLines: string[];
  hunks: UnifiedDiffHunk[];
  id: string;
  oldPath: string;
  path: string;
  patch: string;
}

export interface UnifiedDiffSummary {
  additions: number;
  deletions: number;
  files: UnifiedDiffFile[];
}

export function parseUnifiedDiff(diff: string): UnifiedDiffSummary {
  if (diff.trim().length === 0) {
    return { additions: 0, deletions: 0, files: [] };
  }
  if (!diff.includes("diff --git ") && !diff.includes("@@")) {
    return { additions: 0, deletions: 0, files: [] };
  }

  const files: MutableFile[] = [];
  let currentFile: MutableFile | null = null;
  let currentHunk: MutableHunk | null = null;
  let oldLine = 0;
  let newLine = 0;

  for (const rawLine of diff.split("\n")) {
    if (isPatchWrapperLine(rawLine)) {
      continue;
    }

    if (rawLine.startsWith("diff --git ")) {
      currentFile = createFile(rawLine, files.length);
      files.push(currentFile);
      currentHunk = null;
      continue;
    }

    const patchFilePath = parsePatchFilePath(rawLine);
    if (patchFilePath) {
      currentFile = createPatchFile(rawLine, patchFilePath, files.length);
      files.push(currentFile);
      currentHunk = null;
      continue;
    }

    if (currentFile === null) {
      continue;
    }

    if (rawLine.startsWith("@@")) {
      const range = parseHunkRange(rawLine);
      oldLine = range.oldStart;
      newLine = range.newStart;
      currentHunk = {
        additions: 0,
        deletions: 0,
        header: rawLine,
        id: `${currentFile.id}:hunk-${currentFile.hunks.length}`,
        lines: [],
      };
      currentFile.hunks.push(currentHunk);
      continue;
    }

    if (!currentHunk) {
      currentFile.headerLines.push(rawLine);
      updateFilePath(currentFile, rawLine);
      continue;
    }

    const line = parseDiffLine(rawLine, oldLine, newLine);
    currentHunk.lines.push(line);

    if (line.kind === "add") {
      currentHunk.additions += 1;
      currentFile.additions += 1;
      newLine += 1;
    } else if (line.kind === "delete") {
      currentHunk.deletions += 1;
      currentFile.deletions += 1;
      oldLine += 1;
    } else if (line.kind === "context") {
      oldLine += 1;
      newLine += 1;
    }
  }

  const normalizedFiles = mergeFilesByPath(files)
    .filter((file) => file.headerLines.length > 0 || file.hunks.length > 0)
    .map(finalizeFile);

  return {
    additions: normalizedFiles.reduce((sum, file) => sum + file.additions, 0),
    deletions: normalizedFiles.reduce((sum, file) => sum + file.deletions, 0),
    files: normalizedFiles,
  };
}

function mergeFilesByPath(files: MutableFile[]) {
  const mergedFiles: MutableFile[] = [];
  const fileByPath = new Map<string, MutableFile>();

  for (const file of files) {
    const key = file.path || file.oldPath || file.id;
    const existing = fileByPath.get(key);
    if (!existing) {
      fileByPath.set(key, file);
      mergedFiles.push(file);
      continue;
    }

    existing.additions += file.additions;
    existing.deletions += file.deletions;
    existing.headerLines = mergeHeaderLines(
      existing.headerLines,
      file.headerLines,
    );
    existing.hunks.push(...file.hunks);
  }

  return mergedFiles;
}

function mergeHeaderLines(left: string[], right: string[]) {
  const seen = new Set(left);
  const merged = [...left];
  for (const line of right) {
    if (seen.has(line)) {
      continue;
    }
    seen.add(line);
    merged.push(line);
  }
  return merged;
}

function createPatchFile(
  firstLine: string,
  path: string,
  index: number,
): MutableFile {
  return {
    additions: 0,
    deletions: 0,
    headerLines: [firstLine],
    hunks: [],
    id: `${index}-${path}`,
    oldPath: path,
    path,
  };
}

function createFile(firstLine: string, index: number): MutableFile {
  const paths = parseDiffGitPaths(firstLine);
  const path = paths.newPath || paths.oldPath || `diff-${index + 1}`;
  return {
    additions: 0,
    deletions: 0,
    headerLines: [firstLine],
    hunks: [],
    id: `${index}-${path}`,
    oldPath: paths.oldPath || path,
    path,
  };
}

function finalizeFile(file: MutableFile): UnifiedDiffFile {
  const hunks = file.hunks.map((hunk) => ({
    ...hunk,
    patch: [
      ...file.headerLines,
      hunk.header,
      ...hunk.lines.map((line) => line.raw),
    ]
      .join("\n")
      .trimEnd()
      .concat("\n"),
  }));
  const patch = [...file.headerLines]
    .concat(
      hunks.flatMap((hunk) => [
        hunk.header,
        ...hunk.lines.map((line) => line.raw),
      ]),
    )
    .join("\n")
    .trimEnd()
    .concat("\n");

  return {
    ...file,
    hunks,
    patch,
  };
}

function parseDiffLine(
  raw: string,
  oldLine: number,
  newLine: number,
): UnifiedDiffLine {
  if (raw.startsWith("+") && !raw.startsWith("+++")) {
    return { kind: "add", newLine, raw, text: raw.slice(1) };
  }
  if (raw.startsWith("-") && !raw.startsWith("---")) {
    return { kind: "delete", oldLine, raw, text: raw.slice(1) };
  }
  if (raw.startsWith("\\")) {
    return { kind: "meta", raw, text: raw };
  }
  return { kind: "context", newLine, oldLine, raw, text: raw.slice(1) };
}

function parseHunkRange(header: string) {
  const match = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(header);
  return {
    oldStart: Number(match?.[1] ?? 0),
    newStart: Number(match?.[2] ?? 0),
  };
}

function parseDiffGitPaths(line: string) {
  const match = /^diff --git a\/(.+?) b\/(.+)$/.exec(line);
  return {
    oldPath: normalizeDiffPath(match?.[1] ?? ""),
    newPath: normalizeDiffPath(match?.[2] ?? ""),
  };
}

function updateFilePath(file: MutableFile, line: string) {
  if (line.startsWith("--- ")) {
    file.oldPath = normalizeDiffPath(line.slice(4)) || file.oldPath;
  }
  if (line.startsWith("+++ ")) {
    file.path = normalizeDiffPath(line.slice(4)) || file.path;
  }
}

function normalizeDiffPath(path: string) {
  const trimmed = path.trim();
  if (trimmed === "/dev/null") {
    return "";
  }
  return trimmed.replace(/^a\//, "").replace(/^b\//, "");
}

function isPatchWrapperLine(line: string) {
  return line === "*** Begin Patch" || line === "*** End Patch";
}

function parsePatchFilePath(line: string) {
  const match = /^\*\*\* (?:Update|Add|Delete) File: (.+)$/.exec(line);
  return match?.[1]?.trim() ?? "";
}

type MutableHunk = Omit<UnifiedDiffHunk, "patch">;

interface MutableFile extends Omit<UnifiedDiffFile, "hunks" | "patch"> {
  hunks: MutableHunk[];
}
