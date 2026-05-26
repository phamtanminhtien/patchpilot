import type { AgentSkill, FileIndexEntry } from "@/shared/api";

import { skillDisplayName } from "./skills";

export type ComposerSuggestionKind = "skill" | "folder" | "file";

export interface ComposerSuggestion {
  description: string;
  disabled: boolean;
  group: "Skills" | "Folders" | "Files";
  id: string;
  insertText: string;
  kind: ComposerSuggestionKind;
  label: string;
  path: string;
  searchText: string;
  secondaryLabel: string;
  warning?: string;
}

export function skillSuggestions(skills: AgentSkill[]) {
  return [...skills].sort(compareSkills).map<ComposerSuggestion>((skill) => {
    const displayName = skillDisplayName(skill);
    return {
      description: skill.description || skill.source,
      disabled: !skill.valid,
      group: "Skills",
      id: `skill:${skill.key}`,
      insertText: `[$${skill.key}](${skill.path})`,
      kind: "skill",
      label: displayName,
      path: skill.path,
      searchText: [
        displayName,
        skill.key,
        skill.name,
        skill.path,
        skill.description,
        skill.warning ?? "",
      ].join(" "),
      secondaryLabel: `$${skill.key} ${skill.path}`,
      warning: skill.warning,
    };
  });
}

export function mentionSuggestions(
  skills: AgentSkill[],
  entries: FileIndexEntry[],
  query: string,
) {
  const suggestions = [...skillSuggestions(skills)];
  if (query.trim().length === 0) {
    return suggestions;
  }
  return [
    ...suggestions,
    ...folderSuggestions(entries),
    ...fileSuggestions(entries),
  ];
}

export function filterSuggestions(
  suggestions: ComposerSuggestion[],
  query: string,
) {
  const normalizedQuery = normalize(query);
  if (normalizedQuery.length === 0) {
    return suggestions;
  }
  return suggestions.filter((suggestion) =>
    [
      suggestion.label,
      suggestion.path,
      suggestion.description,
      suggestion.searchText,
      suggestion.warning ?? "",
    ]
      .map(normalize)
      .some((value) => value.includes(normalizedQuery)),
  );
}

function folderSuggestions(entries: FileIndexEntry[]) {
  const folders = new Map<string, string>();
  for (const entry of entries) {
    const parts = entry.path.split("/").filter(Boolean);
    for (let index = 0; index < parts.length - 1; index += 1) {
      const path = `${parts.slice(0, index + 1).join("/")}/`;
      folders.set(path, parts[index] ?? path);
    }
  }

  return Array.from(folders.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map<ComposerSuggestion>(([path, label]) => ({
      description: path,
      disabled: false,
      group: "Folders",
      id: `folder:${path}`,
      insertText: `[${label}](${path})`,
      kind: "folder",
      label,
      path,
      searchText: `${label} ${path}`,
      secondaryLabel: path,
    }));
}

function fileSuggestions(entries: FileIndexEntry[]) {
  return [...entries]
    .sort((left, right) => left.path.localeCompare(right.path))
    .map<ComposerSuggestion>((entry) => {
      const label = entry.path.split("/").filter(Boolean).at(-1) ?? entry.path;
      return {
        description: entry.path,
        disabled: false,
        group: "Files",
        id: `file:${entry.path}`,
        insertText: `[${label}](${entry.path})`,
        kind: "file",
        label,
        path: entry.path,
        searchText: `${label} ${entry.path}`,
        secondaryLabel: entry.path,
      };
    });
}

function compareSkills(left: AgentSkill, right: AgentSkill) {
  const leftRank = skillRank(left);
  const rightRank = skillRank(right);
  if (leftRank !== rightRank) {
    return leftRank - rightRank;
  }
  return skillDisplayName(left).localeCompare(skillDisplayName(right));
}

function skillRank(skill: AgentSkill) {
  if (skill.enabled && skill.valid) {
    return 0;
  }
  if (skill.valid) {
    return 1;
  }
  return 2;
}

function normalize(value: string) {
  return value.trim().toLowerCase();
}
