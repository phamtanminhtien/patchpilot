import type { AgentSkill } from "@/shared/api";

const KNOWN_SKILL_WORDS: Record<string, string> = {
  ai: "AI",
  api: "API",
  css: "CSS",
  db: "DB",
  git: "Git",
  github: "GitHub",
  html: "HTML",
  http: "HTTP",
  https: "HTTPS",
  id: "ID",
  json: "JSON",
  lsp: "LSP",
  mcp: "MCP",
  pr: "PR",
  sse: "SSE",
  svg: "SVG",
  ui: "UI",
  url: "URL",
  yaml: "YAML",
};

export function humanizeSkillName(name: string) {
  return name
    .trim()
    .replace(/[-_:/]+/g, " ")
    .replace(/\s+/g, " ")
    .split(" ")
    .filter(Boolean)
    .map((word) => {
      const normalized = word.toLowerCase();
      return (
        KNOWN_SKILL_WORDS[normalized] ??
        `${normalized.charAt(0).toUpperCase()}${normalized.slice(1)}`
      );
    })
    .join(" ");
}

export function skillDisplayName(skill: AgentSkill) {
  return humanizeSkillName(skill.name || skill.key);
}
