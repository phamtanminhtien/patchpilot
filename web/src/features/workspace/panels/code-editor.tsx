import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { css } from "@codemirror/lang-css";
import { go } from "@codemirror/lang-go";
import { html } from "@codemirror/lang-html";
import { javascript } from "@codemirror/lang-javascript";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import {
  bracketMatching,
  foldGutter,
  HighlightStyle,
  indentOnInput,
  type LanguageSupport,
  syntaxHighlighting,
} from "@codemirror/language";
import { highlightSelectionMatches, searchKeymap } from "@codemirror/search";
import { EditorState, type Extension } from "@codemirror/state";
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
} from "@codemirror/view";
import { tags } from "@lezer/highlight";
import { useEffect, useRef } from "react";

import { cn } from "@/shared/ui";

interface CodeEditorProps {
  ariaLabel: string;
  className?: string;
  onChange: (value: string) => void;
  onSave: () => void;
  path: string;
  value: string;
}

export function CodeEditor({
  ariaLabel,
  className,
  onChange,
  onSave,
  path,
  value,
}: CodeEditorProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    onSaveRef.current = onSave;
  }, [onSave]);

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const extensions = [
      ...baseExtensions,
      languageForPath(path),
      keymap.of([
        {
          key: "Mod-s",
          preventDefault: true,
          run: () => {
            onSaveRef.current();
            return true;
          },
        },
        indentWithTab,
        ...searchKeymap,
        ...historyKeymap,
        ...defaultKeymap,
      ]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          onChangeRef.current(update.state.doc.toString());
        }
      }),
    ];

    const view = new EditorView({
      doc: "",
      extensions,
      parent: containerRef.current,
    });
    view.contentDOM.setAttribute("aria-label", ariaLabel);
    view.contentDOM.setAttribute("role", "textbox");
    view.contentDOM.setAttribute("aria-multiline", "true");
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [ariaLabel, path]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) {
      return;
    }
    const currentValue = view.state.doc.toString();
    if (currentValue === value) {
      return;
    }
    view.dispatch({
      changes: { from: 0, to: currentValue.length, insert: value },
    });
  }, [value]);

  return (
    <div
      className={cn("workspace-code-editor min-h-0 overflow-hidden", className)}
      ref={containerRef}
    />
  );
}

const editorTheme = EditorView.theme({
  "&": {
    backgroundColor: "var(--pp-bg-panel)",
    color: "var(--pp-color-ink)",
    fontFamily: "var(--pp-font-code)",
    fontSize: "0.75rem",
    height: "100%",
  },
  ".cm-activeLine": {
    backgroundColor: "color-mix(in srgb, var(--pp-bg-hover) 36%, transparent)",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "color-mix(in srgb, var(--pp-bg-hover) 42%, transparent)",
    color: "var(--pp-color-ink)",
  },
  ".cm-content": {
    caretColor: "var(--pp-color-accent)",
    minHeight: "100%",
    padding: "0.75rem 0",
  },
  ".cm-cursor": {
    borderLeftColor: "var(--pp-color-accent)",
  },
  ".cm-focused": {
    outline: "none",
  },
  ".cm-gutters": {
    backgroundColor: "var(--pp-bg-surface)",
    borderRight:
      "1px solid color-mix(in srgb, var(--pp-color-line) 55%, transparent)",
    color: "var(--pp-color-muted)",
  },
  ".cm-line": {
    lineHeight: "1.25rem",
    padding: "0 0.75rem",
  },
  ".cm-scroller": {
    fontFamily: "var(--pp-font-code)",
    height: "100%",
    overflow: "auto",
  },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
    {
      backgroundColor:
        "color-mix(in srgb, var(--pp-color-accent) 24%, transparent)",
    },
});

const patchPilotHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--pp-code-keyword)" },
  { tag: tags.operatorKeyword, color: "var(--pp-code-keyword)" },
  { tag: tags.atom, color: "var(--pp-code-atom)" },
  { tag: tags.bool, color: "var(--pp-code-bool)" },
  { tag: tags.number, color: "var(--pp-code-number)" },
  { tag: tags.string, color: "var(--pp-code-string)" },
  { tag: tags.special(tags.string), color: "var(--pp-code-literal)" },
  { tag: tags.regexp, color: "var(--pp-code-literal)" },
  { tag: tags.escape, color: "var(--pp-code-literal)" },
  { tag: tags.comment, color: "var(--pp-code-comment)", fontStyle: "italic" },
  { tag: tags.variableName, color: "var(--pp-code-variable)" },
  { tag: tags.definition(tags.variableName), color: "var(--pp-code-def)" },
  { tag: tags.function(tags.variableName), color: "var(--pp-code-def)" },
  { tag: tags.propertyName, color: "var(--pp-code-property)" },
  { tag: tags.definition(tags.propertyName), color: "var(--pp-code-property)" },
  { tag: tags.typeName, color: "var(--pp-code-type)" },
  { tag: tags.className, color: "var(--pp-code-type)" },
  { tag: tags.tagName, color: "var(--pp-code-tag)" },
  { tag: tags.attributeName, color: "var(--pp-code-property)" },
  { tag: tags.punctuation, color: "var(--pp-code-punctuation)" },
  { tag: tags.operator, color: "var(--pp-code-operator)" },
  { tag: tags.meta, color: "var(--pp-code-meta)" },
  { tag: tags.link, color: "var(--pp-code-link)", textDecoration: "underline" },
  { tag: tags.heading, color: "var(--pp-code-keyword)", fontWeight: "600" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strong, fontWeight: "600" },
]);

const baseExtensions: Extension[] = [
  lineNumbers(),
  foldGutter(),
  highlightActiveLineGutter(),
  history(),
  drawSelection(),
  indentOnInput(),
  bracketMatching(),
  syntaxHighlighting(patchPilotHighlightStyle, { fallback: true }),
  highlightActiveLine(),
  highlightSelectionMatches(),
  EditorState.tabSize.of(2),
  editorTheme,
];

function languageForPath(path: string): LanguageSupport | [] {
  const extension = path.split(".").pop()?.toLowerCase() ?? "";
  switch (extension) {
    case "cjs":
    case "js":
    case "jsx":
    case "mjs":
      return javascript({ jsx: true });
    case "ts":
    case "tsx":
      return javascript({ jsx: true, typescript: true });
    case "json":
      return json();
    case "html":
    case "htm":
      return html();
    case "css":
      return css();
    case "md":
    case "mdx":
    case "markdown":
      return markdown();
    case "go":
      return go();
    default:
      return [];
  }
}
