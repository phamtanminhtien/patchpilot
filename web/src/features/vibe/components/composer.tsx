import { type JSONContent, mergeAttributes, Node } from "@tiptap/core";
import Document from "@tiptap/extension-document";
import Paragraph from "@tiptap/extension-paragraph";
import Placeholder from "@tiptap/extension-placeholder";
import Text from "@tiptap/extension-text";
import {
  type Editor as TiptapEditor,
  EditorContent,
  useEditor,
} from "@tiptap/react";
import {
  ArrowUp,
  ChevronDown,
  FileText,
  Folder,
  Loader2,
  Puzzle,
  ShieldCheck,
  Square,
} from "lucide-react";
import {
  type FormEvent,
  forwardRef,
  type KeyboardEvent,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type {
  AgentModel,
  AgentReasoningEffort,
  AgentSkill,
  FileIndexEntry,
  PatchWorkspacePermissionsRequest,
  PermissionMode,
  WorkspacePermissions,
} from "@/shared/api";
import {
  Button,
  cn,
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
  Select,
  Switch,
} from "@/shared/ui";

import {
  type ComposerSuggestion,
  filterSuggestions,
  mentionSuggestions,
  skillSuggestions,
} from "../lib/composer-suggestions";
import { humanizeSkillName } from "../lib/skills";
import { agentModels, reasoningEfforts } from "../vibe-options";

type TriggerKind = "slash" | "mention";

interface TiptapTrigger {
  from: number;
  kind: TriggerKind;
  query: string;
  to: number;
}

interface ComposerEditorHandle {
  focus: () => void;
  insertSuggestion: (
    trigger: TiptapTrigger,
    suggestion: ComposerSuggestion,
  ) => void;
}

interface ComposerLinkToken {
  display: string;
  kind: ComposerSuggestion["kind"];
  label: string;
  markdown: string;
  path: string;
}

export function Composer({
  activeRun,
  error,
  fileIndexEntries,
  fileIndexError,
  isFileIndexLoading,
  isPending,
  isSkillsLoading,
  isStopping,
  model,
  onModelChange,
  onPermissionsChange,
  onPromptChange,
  onReasoningEffortChange,
  onStop,
  onSubmit,
  prompt,
  promptResetKey,
  permissions,
  permissionsError,
  permissionsLoading,
  permissionsSaving,
  reasoningEffort,
  skills,
  skillsError,
  workspaceReady,
}: {
  activeRun: boolean;
  error?: string;
  fileIndexEntries: FileIndexEntry[];
  fileIndexError?: string;
  isFileIndexLoading: boolean;
  isPending: boolean;
  isSkillsLoading: boolean;
  isStopping: boolean;
  model: AgentModel;
  onModelChange: (model: AgentModel) => void;
  onPermissionsChange: (permissions: PatchWorkspacePermissionsRequest) => void;
  onPromptChange: (prompt: string) => void;
  onReasoningEffortChange: (effort: AgentReasoningEffort) => void;
  onStop: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  prompt: string;
  promptResetKey: number;
  permissions: WorkspacePermissions;
  permissionsError?: string;
  permissionsLoading: boolean;
  permissionsSaving: boolean;
  reasoningEffort: AgentReasoningEffort;
  skills: AgentSkill[];
  skillsError?: string;
  workspaceReady: boolean;
}) {
  const editorRef = useRef<ComposerEditorHandle | null>(null);
  const [promptContentState, setPromptContentState] = useState({
    hasContent: prompt.trim().length > 0,
    resetKey: promptResetKey,
  });
  const [rawTrigger, setRawTrigger] = useState<TiptapTrigger | null>(null);
  const [activeSuggestionId, setActiveSuggestionId] = useState("");
  const [dismissedTriggerKey, setDismissedTriggerKey] = useState("");
  const hasPromptContent =
    promptContentState.resetKey === promptResetKey
      ? promptContentState.hasContent
      : prompt.trim().length > 0;
  const effectiveRawTrigger =
    promptContentState.resetKey === promptResetKey ? rawTrigger : null;
  const rawTriggerKey = effectiveRawTrigger
    ? `${promptResetKey}:${triggerKey(effectiveRawTrigger)}`
    : "";
  const trigger =
    effectiveRawTrigger && rawTriggerKey !== dismissedTriggerKey
      ? effectiveRawTrigger
      : null;
  const suggestions = useMemo(
    () => suggestionsForTrigger(trigger, skills, fileIndexEntries),
    [fileIndexEntries, skills, trigger],
  );
  const selectableSuggestions = suggestions.filter(
    (suggestion) => !suggestion.disabled,
  );
  const activeSuggestion =
    selectableSuggestions.find(
      (suggestion) => suggestion.id === activeSuggestionId,
    ) ?? selectableSuggestions[0];
  const isSuggestionOpen = trigger !== null;

  function insertSuggestion(suggestion: ComposerSuggestion) {
    if (trigger === null || suggestion.disabled) {
      return;
    }
    editorRef.current?.insertSuggestion(trigger, suggestion);
    setDismissedTriggerKey("");
  }

  function handlePromptKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (!isSuggestionOpen) {
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      if (trigger) {
        setDismissedTriggerKey(rawTriggerKey);
      }
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const direction = event.key === "ArrowDown" ? 1 : -1;
      const currentIndex = activeSuggestion
        ? selectableSuggestions.findIndex(
            (suggestion) => suggestion.id === activeSuggestion.id,
          )
        : 0;
      const nextIndex = wrapIndex(
        currentIndex + direction,
        selectableSuggestions.length,
      );
      setActiveSuggestionId(selectableSuggestions[nextIndex]?.id ?? "");
      return;
    }
    if (event.key === "Enter" || event.key === "Tab") {
      if (activeSuggestion) {
        event.preventDefault();
        insertSuggestion(activeSuggestion);
      }
    }
  }

  return (
    <form className="relative grid gap-2" onSubmit={onSubmit}>
      {isSuggestionOpen ? (
        <ComposerSuggestionList
          activeSuggestion={activeSuggestion}
          fileIndexError={fileIndexError}
          hasMentionQuery={
            trigger.kind === "mention" && trigger.query.trim().length > 0
          }
          isFileIndexLoading={isFileIndexLoading}
          isSkillsLoading={isSkillsLoading}
          onInsert={insertSuggestion}
          onSetActive={(suggestion) => {
            setActiveSuggestionId(suggestion.id);
          }}
          skillsError={skillsError}
          suggestions={suggestions}
          triggerKind={trigger.kind}
        />
      ) : null}
      <div className="vibe-composer bg-composer grid overflow-hidden rounded-xl transition">
        <label className="sr-only" htmlFor="agent-prompt">
          Ask AI
        </label>
        <ComposerEditor
          ariaLabel="Ask AI"
          disabled={!workspaceReady}
          id="agent-prompt"
          onChange={(nextPrompt) => {
            const nextHasContent = nextPrompt.trim().length > 0;
            setPromptContentState((current) => {
              if (
                current.resetKey === promptResetKey &&
                current.hasContent === nextHasContent
              ) {
                return current;
              }
              return {
                hasContent: nextHasContent,
                resetKey: promptResetKey,
              };
            });
            onPromptChange(nextPrompt);
            setDismissedTriggerKey("");
          }}
          onKeyDown={handlePromptKeyDown}
          onTriggerChange={setRawTrigger}
          placeholder="Ask for follow-up changes"
          ref={editorRef}
          resetKey={promptResetKey}
          skills={skills}
          value={prompt}
        />
        <div className="bg-composer-bar flex min-w-0 flex-col gap-2 px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <PermissionMenu
              disabled={!workspaceReady || permissionsLoading}
              error={permissionsError}
              isLoading={permissionsLoading}
              isSaving={permissionsSaving}
              onChange={onPermissionsChange}
              permissions={permissions}
            />
          </div>
          <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2">
            <Select
              label="Model"
              onValueChange={(value) => onModelChange(value as AgentModel)}
              options={agentModels.map((option) => ({ value: option }))}
              size="tiny"
              triggerClassName="w-full"
              value={model}
            />
            <Select
              label="Reasoning"
              onValueChange={(value) =>
                onReasoningEffortChange(value as AgentReasoningEffort)
              }
              options={reasoningEfforts.map((option) => ({ value: option }))}
              size="tiny"
              triggerClassName="w-full"
              value={reasoningEffort}
            />
            <Button
              aria-label={activeRun ? "Stop run" : "Start run"}
              disabled={
                activeRun
                  ? isStopping
                  : !workspaceReady || !hasPromptContent || isPending
              }
              icon={
                isPending || isStopping ? (
                  <Loader2 className="size-4! animate-spin" />
                ) : activeRun ? (
                  <Square className="size-4!" />
                ) : (
                  <ArrowUp className="size-4!" />
                )
              }
              className="size-8 shrink-0 rounded-xl"
              onClick={activeRun ? onStop : undefined}
              size="icon"
              type={activeRun ? "button" : "submit"}
            />
          </div>
        </div>
      </div>
      {error ? (
        <p className="text-warning text-xs font-medium">{error}</p>
      ) : null}
    </form>
  );
}

const permissionModes: PermissionMode[] = ["safe", "balanced", "autonomous"];

const permissionPresets: Record<PermissionMode, WorkspacePermissions> = {
  autonomous: {
    editFiles: true,
    gitOperations: true,
    mode: "autonomous",
    runCommands: true,
  },
  balanced: {
    editFiles: true,
    gitOperations: true,
    mode: "balanced",
    runCommands: true,
  },
  safe: {
    editFiles: true,
    gitOperations: true,
    mode: "safe",
    runCommands: true,
  },
};

function PermissionMenu({
  disabled,
  error,
  isLoading,
  isSaving,
  onChange,
  permissions,
}: {
  disabled: boolean;
  error?: string;
  isLoading: boolean;
  isSaving: boolean;
  onChange: (permissions: PatchWorkspacePermissionsRequest) => void;
  permissions: WorkspacePermissions;
}) {
  const modeLabel = modeDisplayName(permissions.mode);
  return (
    <PopoverRoot>
      <PopoverTrigger asChild>
        <button
          className="bg-surface hover:bg-hover text-muted inline-flex min-h-7 min-w-0 cursor-pointer items-center gap-1.5 rounded-xl px-2 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          type="button"
        >
          <ShieldCheck aria-hidden="true" className="size-3.5" />
          <span className="truncate">
            {isLoading ? "Loading permissions" : `${modeLabel} permissions`}
          </span>
          {isSaving ? (
            <Loader2 aria-hidden="true" className="size-3.5 animate-spin" />
          ) : (
            <ChevronDown aria-hidden="true" className="size-3.5" />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 gap-3 p-3">
        <div
          aria-label="Permission mode"
          className="bg-surface grid grid-cols-3 gap-1 rounded-lg p-1"
          role="group"
        >
          {permissionModes.map((mode) => (
            <button
              aria-pressed={permissions.mode === mode}
              className={cn(
                "text-muted hover:text-ink rounded-md px-2 py-1.5 text-xs font-semibold transition",
                permissions.mode === mode && "bg-panel text-ink",
              )}
              key={mode}
              onClick={() => onChange(permissionPresets[mode])}
              type="button"
            >
              {modeDisplayName(mode)}
            </button>
          ))}
        </div>
        <div className="grid gap-2">
          <PermissionSwitch
            checked={permissions.editFiles}
            label="Edit files"
            onCheckedChange={(checked) => onChange({ editFiles: checked })}
          />
          <PermissionSwitch
            checked={permissions.runCommands}
            label="Run commands"
            onCheckedChange={(checked) => onChange({ runCommands: checked })}
          />
          <PermissionSwitch
            checked={permissions.gitOperations}
            label="Git operations"
            onCheckedChange={(checked) => onChange({ gitOperations: checked })}
          />
        </div>
        <div className="border-border grid gap-1 border-t pt-3 text-xs">
          {approvalRules(permissions).map((rule) => (
            <div
              className="grid grid-cols-[5.5rem_minmax(0,1fr)] gap-2"
              key={rule.label}
            >
              <span className="text-muted font-medium">{rule.label}</span>
              <span className="text-ink">{rule.value}</span>
            </div>
          ))}
        </div>
        {error ? (
          <p className="text-warning text-xs font-medium">{error}</p>
        ) : null}
      </PopoverContent>
    </PopoverRoot>
  );
}

function PermissionSwitch({
  checked,
  label,
  onCheckedChange,
}: {
  checked: boolean;
  label: string;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm font-medium">
      <span>{label}</span>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </label>
  );
}

function approvalRules(permissions: WorkspacePermissions) {
  const edits = permissions.editFiles
    ? permissions.mode === "autonomous"
      ? "apply_patch auto-runs"
      : "apply_patch needs approval"
    : "apply_patch is blocked";
  const commands = !permissions.runCommands
    ? "run_command is blocked"
    : permissions.mode === "safe"
      ? "all commands need approval"
      : permissions.mode === "autonomous"
        ? "safe and confirmation commands auto-run"
        : "safe commands auto-run; confirmation commands need approval";
  const git = permissions.gitOperations
    ? "git commands follow command rules"
    : "git commands are blocked";
  return [
    { label: "Files", value: edits },
    { label: "Commands", value: commands },
    { label: "Git", value: git },
    { label: "MCP", value: "MCP tools need approval" },
  ];
}

function modeDisplayName(mode: PermissionMode) {
  switch (mode) {
    case "safe":
      return "Safe";
    case "autonomous":
      return "Autonomous";
    default:
      return "Balanced";
  }
}

const ComposerEditor = forwardRef<
  ComposerEditorHandle,
  {
    ariaLabel: string;
    disabled: boolean;
    id: string;
    onChange: (value: string) => void;
    onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
    onTriggerChange: (trigger: TiptapTrigger | null) => void;
    placeholder: string;
    resetKey: number;
    skills: AgentSkill[];
    value: string;
  }
>(function ComposerEditor(
  {
    ariaLabel,
    disabled,
    id,
    onChange,
    onKeyDown,
    onTriggerChange,
    placeholder,
    resetKey,
    skills,
    value,
  },
  ref,
) {
  const skillsByKey = useMemo(() => skillDisplayNamesByKey(skills), [skills]);
  const appliedResetKeyRef = useRef<number | null>(null);
  const editor = useEditor({
    content: promptToTiptapContent(value, skillsByKey),
    editable: !disabled,
    editorProps: {
      attributes: {
        "aria-label": ariaLabel,
        class:
          "min-h-24 w-full px-4 pt-4 pb-3 text-sm leading-6 text-ink outline-none focus:outline-none whitespace-pre-wrap break-words",
        id,
        role: "textbox",
      },
    },
    extensions: [
      Document,
      Paragraph,
      Text,
      Placeholder.configure({ placeholder }),
      ComposerLinkTokenNode,
    ],
    immediatelyRender: false,
    onSelectionUpdate: ({ editor: nextEditor }) => {
      onTriggerChange(activeTrigger(nextEditor));
    },
    onUpdate: ({ editor: nextEditor }) => {
      const nextValue = serializeTiptapContent(nextEditor);
      onChange(nextValue);
      onTriggerChange(activeTrigger(nextEditor));
    },
  });

  useEffect(() => {
    if (!editor || appliedResetKeyRef.current === resetKey) {
      return;
    }
    appliedResetKeyRef.current = resetKey;
    editor.commands.setContent(promptToTiptapContent(value, skillsByKey), {
      emitUpdate: false,
    });
    onTriggerChange(activeTrigger(editor));
  }, [editor, onTriggerChange, resetKey, skillsByKey, value]);

  useEffect(() => {
    editor?.setEditable(!disabled);
  }, [disabled, editor]);

  useImperativeHandle(
    ref,
    () => ({
      focus() {
        editor?.commands.focus();
      },
      insertSuggestion(trigger, suggestion) {
        if (!editor || suggestion.disabled) {
          return;
        }
        const token = tokenFromSuggestion(suggestion);
        editor
          .chain()
          .focus()
          .deleteRange({ from: trigger.from, to: trigger.to })
          .insertContent({ type: "composerLinkToken", attrs: token })
          .run();
        onTriggerChange(activeTrigger(editor));
      },
    }),
    [editor, onTriggerChange],
  );

  return (
    <EditorContent
      className={cn(
        "min-h-24",
        "[&_.ProseMirror-focused]:outline-none",
        "[&_.is-editor-empty:first-child::before]:text-muted [&_.is-editor-empty:first-child::before]:pointer-events-none [&_.is-editor-empty:first-child::before]:float-left [&_.is-editor-empty:first-child::before]:h-0 [&_.is-editor-empty:first-child::before]:content-[attr(data-placeholder)]",
        disabled ? "cursor-not-allowed opacity-60" : "",
      )}
      editor={editor}
      onKeyDownCapture={(event) => {
        onKeyDown(event);
        if (event.defaultPrevented || !editor) {
          return;
        }
        if (event.key === "Enter" && !event.nativeEvent.isComposing) {
          event.preventDefault();
          if (event.metaKey || event.ctrlKey) {
            editor.commands.splitBlock();
            onTriggerChange(activeTrigger(editor));
            return;
          }
          event.currentTarget.closest("form")?.requestSubmit();
          return;
        }
        if (deleteAdjacentComposerToken(editor, event.key)) {
          event.preventDefault();
          onTriggerChange(activeTrigger(editor));
        }
      }}
    />
  );
});

function ComposerSuggestionList({
  activeSuggestion,
  fileIndexError,
  hasMentionQuery,
  isFileIndexLoading,
  isSkillsLoading,
  onInsert,
  onSetActive,
  skillsError,
  suggestions,
  triggerKind,
}: {
  activeSuggestion?: ComposerSuggestion;
  fileIndexError?: string;
  hasMentionQuery: boolean;
  isFileIndexLoading: boolean;
  isSkillsLoading: boolean;
  onInsert: (suggestion: ComposerSuggestion) => void;
  onSetActive: (suggestion: ComposerSuggestion) => void;
  skillsError?: string;
  suggestions: ComposerSuggestion[];
  triggerKind: TriggerKind;
}) {
  const optionRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const groups = groupedSuggestions(suggestions);
  const showSkillLoading = isSkillsLoading && suggestions.length === 0;
  const showFileLoading =
    triggerKind === "mention" && hasMentionQuery && isFileIndexLoading;
  const showEmpty = !showSkillLoading && suggestions.length === 0;

  useLayoutEffect(() => {
    if (!activeSuggestion) {
      return;
    }
    optionRefs.current[activeSuggestion.id]?.scrollIntoView({
      block: "nearest",
    });
  }, [activeSuggestion]);

  return (
    <div
      aria-label={
        triggerKind === "slash" ? "Slash suggestions" : "Mention suggestions"
      }
      className="border-line/40 bg-panel absolute inset-x-0 bottom-full z-20 mb-1.5 grid max-h-[min(14rem,calc(100vh-12rem))] min-w-0 overflow-auto rounded-md border p-1 shadow-md"
      role="listbox"
    >
      {showSkillLoading ? <SuggestionStatus text="Loading skills" /> : null}
      {skillsError ? (
        <SuggestionStatus tone="warning" text={skillsError} />
      ) : null}
      {showFileLoading ? <SuggestionStatus text="Loading files" /> : null}
      {triggerKind === "mention" && hasMentionQuery && fileIndexError ? (
        <SuggestionStatus tone="warning" text={fileIndexError} />
      ) : null}
      {showEmpty ? (
        <SuggestionStatus
          text={
            triggerKind === "slash"
              ? "No matching skills."
              : "No matching mentions."
          }
        />
      ) : null}
      {groups.map(([group, items]) => (
        <div className="grid min-w-0" key={group}>
          <div className="text-muted px-2 pt-1.5 pb-0.5 text-[0.6875rem] font-semibold uppercase">
            {group}
          </div>
          {items.map((suggestion) => (
            <SuggestionRow
              isActive={activeSuggestion?.id === suggestion.id}
              key={suggestion.id}
              onInsert={onInsert}
              onSetActive={onSetActive}
              optionRef={(element) => {
                optionRefs.current[suggestion.id] = element;
              }}
              suggestion={suggestion}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

function SuggestionRow({
  isActive,
  onInsert,
  onSetActive,
  optionRef,
  suggestion,
}: {
  isActive: boolean;
  onInsert: (suggestion: ComposerSuggestion) => void;
  onSetActive: (suggestion: ComposerSuggestion) => void;
  optionRef: (element: HTMLButtonElement | null) => void;
  suggestion: ComposerSuggestion;
}) {
  const Icon =
    suggestion.kind === "skill"
      ? Puzzle
      : suggestion.kind === "folder"
        ? Folder
        : FileText;

  return (
    <button
      aria-disabled={suggestion.disabled}
      aria-selected={isActive}
      aria-label={`${suggestion.label} ${suggestion.secondaryLabel} ${
        suggestion.warning || suggestion.description || suggestion.path
      }`}
      className="hover:bg-hover aria-selected:bg-accent-soft grid min-h-9 min-w-0 cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-1 text-left transition disabled:cursor-not-allowed disabled:opacity-55"
      disabled={suggestion.disabled}
      onClick={() => onInsert(suggestion)}
      onMouseDown={(event) => event.preventDefault()}
      onMouseEnter={() => {
        if (!suggestion.disabled) {
          onSetActive(suggestion);
        }
      }}
      ref={optionRef}
      role="option"
      type="button"
    >
      <span className="bg-hover text-muted grid size-6 shrink-0 place-items-center rounded-sm">
        <Icon aria-hidden="true" className="size-3.5" />
      </span>
      <span className="flex min-w-0 items-baseline gap-2">
        <span className="text-ink min-w-0 truncate text-sm font-medium">
          {suggestion.label}
        </span>
        <span className="text-muted min-w-0 truncate text-xs">
          {suggestion.secondaryLabel}
        </span>
      </span>
    </button>
  );
}

function SuggestionStatus({
  text,
  tone = "muted",
}: {
  text: string;
  tone?: "muted" | "warning";
}) {
  return (
    <p
      className={`px-2 py-3 text-sm ${
        tone === "warning" ? "text-warning" : "text-muted"
      }`}
    >
      {text}
    </p>
  );
}

function suggestionsForTrigger(
  trigger: TiptapTrigger | null,
  skills: AgentSkill[],
  fileIndexEntries: FileIndexEntry[],
) {
  if (trigger === null) {
    return [];
  }
  const suggestions =
    trigger.kind === "slash"
      ? skillSuggestions(skills)
      : mentionSuggestions(skills, fileIndexEntries, trigger.query);
  return filterSuggestions(suggestions, trigger.query);
}

function groupedSuggestions(suggestions: ComposerSuggestion[]) {
  const groups = new Map<ComposerSuggestion["group"], ComposerSuggestion[]>();
  for (const suggestion of suggestions) {
    groups.set(suggestion.group, [
      ...(groups.get(suggestion.group) ?? []),
      suggestion,
    ]);
  }
  return Array.from(groups.entries());
}

function activeTrigger(editor: TiptapEditor): TiptapTrigger | null {
  const { selection } = editor.state;
  if (!selection.empty) {
    return null;
  }
  const caretPosition = selection.from;
  const prefix = editor.state.doc.textBetween(
    Math.max(0, caretPosition - 160),
    caretPosition,
    "\n",
    " ",
  );
  const match = /(^|\s)([/@])([^\s]*)$/.exec(prefix);
  if (!match || match.index === undefined) {
    return null;
  }
  const token = match[2];
  const query = match[3] ?? "";
  if (token !== "/" && token !== "@") {
    return null;
  }
  const triggerLength = token.length + query.length;
  return {
    from: caretPosition - triggerLength,
    kind: token === "/" ? "slash" : "mention",
    query,
    to: caretPosition,
  };
}

function triggerKey(trigger: TiptapTrigger) {
  return `${trigger.kind}:${trigger.from}:${trigger.to}:${trigger.query}`;
}

function wrapIndex(index: number, length: number) {
  if (length === 0) {
    return 0;
  }
  if (index < 0) {
    return length - 1;
  }
  if (index >= length) {
    return 0;
  }
  return index;
}

function deleteAdjacentComposerToken(editor: TiptapEditor, key: string) {
  if (key !== "Backspace" && key !== "Delete") {
    return false;
  }
  const { selection } = editor.state;
  if (!selection.empty) {
    return false;
  }
  const resolvedPosition = selection.$from;
  const adjacentNode =
    key === "Backspace"
      ? resolvedPosition.nodeBefore
      : resolvedPosition.nodeAfter;
  if (adjacentNode?.type.name !== "composerLinkToken") {
    return false;
  }
  const from =
    key === "Backspace"
      ? selection.from - adjacentNode.nodeSize
      : selection.from;
  editor.commands.deleteRange({ from, to: from + adjacentNode.nodeSize });
  return true;
}

const ComposerLinkTokenNode = Node.create({
  name: "composerLinkToken",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      display: tokenNodeAttribute("display"),
      kind: tokenNodeAttribute("kind", "file"),
      label: tokenNodeAttribute("label"),
      markdown: tokenNodeAttribute("markdown"),
      path: tokenNodeAttribute("path"),
    };
  },

  parseHTML() {
    return [{ tag: "span[data-composer-link-token]" }];
  },

  renderHTML({ HTMLAttributes, node }) {
    const display = tokenAttribute(node.attrs, "display");
    const kind = tokenAttribute(node.attrs, "kind");
    const label = tokenAttribute(node.attrs, "label");
    const markdown = tokenAttribute(node.attrs, "markdown");
    const path = tokenAttribute(node.attrs, "path");
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-composer-link-token": "true",
        "data-display": display,
        "data-kind": kind,
        "data-label": label,
        "data-markdown": markdown,
        "data-path": path,
        class:
          "bg-accent-soft text-accent mx-0.5 inline-flex max-w-full items-center rounded-sm px-1.5 py-0.5 align-baseline text-xs font-medium",
        contenteditable: "false",
      }),
      display || label,
    ];
  },

  renderText({ node }) {
    return tokenAttribute(node.attrs, "markdown");
  },
});

function tokenNodeAttribute(name: keyof ComposerLinkToken, defaultValue = "") {
  return {
    default: defaultValue,
    parseHTML: (element: HTMLElement) =>
      element.getAttribute(`data-${name}`) ?? defaultValue,
    renderHTML: () => ({}),
  };
}

function tokenAttribute(attrs: unknown, key: keyof ComposerLinkToken) {
  if (attrs === null || typeof attrs !== "object" || !(key in attrs)) {
    return "";
  }
  const value = attrs[key as keyof typeof attrs];
  return typeof value === "string" ? value : "";
}

function promptToTiptapContent(
  value: string,
  skillNameByKey: Map<string, string>,
): JSONContent {
  const lines = value.split("\n");
  return {
    type: "doc",
    content: lines.map((line) => ({
      type: "paragraph",
      content: line.length > 0 ? parsePromptLine(line, skillNameByKey) : [],
    })),
  };
}

function parsePromptLine(value: string, skillNameByKey: Map<string, string>) {
  const tokens: JSONContent[] = [];
  const linkPattern = /\[([^\]\n]+)\]\(([^)\n]+)\)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = linkPattern.exec(value)) !== null) {
    const [plain, label, path] = match;
    if (match.index > lastIndex) {
      tokens.push({ type: "text", text: value.slice(lastIndex, match.index) });
    }
    tokens.push({
      type: "composerLinkToken",
      attrs: tokenFromMarkdown(label ?? "", path ?? "", plain, skillNameByKey),
    });
    lastIndex = match.index + plain.length;
  }

  if (lastIndex < value.length) {
    tokens.push({ type: "text", text: value.slice(lastIndex) });
  }

  return tokens;
}

function serializeTiptapContent(editor: TiptapEditor) {
  const paragraphs: string[] = [];
  editor.state.doc.forEach((paragraph) => {
    let text = "";
    paragraph.forEach((node) => {
      if (node.isText) {
        text += node.text ?? "";
        return;
      }
      if (node.type.name === "composerLinkToken") {
        text += node.attrs.markdown ?? "";
      }
    });
    paragraphs.push(text);
  });
  return paragraphs.join("\n");
}

function tokenFromSuggestion(
  suggestion: ComposerSuggestion,
): ComposerLinkToken {
  return {
    display:
      suggestion.kind === "folder" ? `${suggestion.label}/` : suggestion.label,
    kind: suggestion.kind,
    label: suggestion.label,
    markdown: suggestion.insertText,
    path: suggestion.path,
  };
}

function tokenFromMarkdown(
  label: string,
  path: string,
  markdown: string,
  skillNameByKey: Map<string, string>,
): ComposerLinkToken {
  const kind = label.startsWith("$")
    ? "skill"
    : path.endsWith("/")
      ? "folder"
      : "file";
  return {
    display: displayForToken(label, path, skillNameByKey),
    kind,
    label,
    markdown,
    path,
  };
}

function displayForToken(
  label: string,
  path: string,
  skillNameByKey: Map<string, string>,
) {
  if (label.startsWith("$")) {
    const key = label.slice(1);
    return skillNameByKey.get(key) ?? humanizeSkillName(key);
  }
  return path.endsWith("/") ? `${label}/` : label;
}

function skillDisplayNamesByKey(skills: AgentSkill[]) {
  return new Map(
    skills.map((skill) => [
      skill.key,
      humanizeSkillName(skill.name || skill.key),
    ]),
  );
}
