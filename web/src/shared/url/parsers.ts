import { parseAsString, parseAsStringLiteral } from "nuqs";

export const modeParser = parseAsStringLiteral([
  "vibe",
  "workspace",
] as const).withDefault("vibe");
export const panelParser = parseAsStringLiteral([
  "files",
  "git",
  "commands",
  "preview",
] as const).withDefault("files");
export const pathParser = parseAsString.withDefault("");
export const conversationIdParser = parseAsString.withDefault("");
export const workspaceIdParser = parseAsString.withDefault("");
