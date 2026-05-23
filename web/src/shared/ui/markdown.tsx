import { Check, Copy } from "lucide-react";
import { Children, isValidElement, type ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";
import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import bash from "react-syntax-highlighter/dist/esm/languages/prism/bash";
import css from "react-syntax-highlighter/dist/esm/languages/prism/css";
import diff from "react-syntax-highlighter/dist/esm/languages/prism/diff";
import docker from "react-syntax-highlighter/dist/esm/languages/prism/docker";
import go from "react-syntax-highlighter/dist/esm/languages/prism/go";
import javascript from "react-syntax-highlighter/dist/esm/languages/prism/javascript";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import jsx from "react-syntax-highlighter/dist/esm/languages/prism/jsx";
import markdown from "react-syntax-highlighter/dist/esm/languages/prism/markdown";
import markup from "react-syntax-highlighter/dist/esm/languages/prism/markup";
import sql from "react-syntax-highlighter/dist/esm/languages/prism/sql";
import tsx from "react-syntax-highlighter/dist/esm/languages/prism/tsx";
import typescript from "react-syntax-highlighter/dist/esm/languages/prism/typescript";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import remarkGfm from "remark-gfm";

import { FileIcon } from "@/shared/file-icons";

import { cn } from "./class-name";

SyntaxHighlighter.registerLanguage("bash", bash);
SyntaxHighlighter.registerLanguage("css", css);
SyntaxHighlighter.registerLanguage("diff", diff);
SyntaxHighlighter.registerLanguage("docker", docker);
SyntaxHighlighter.registerLanguage("go", go);
SyntaxHighlighter.registerLanguage("javascript", javascript);
SyntaxHighlighter.registerLanguage("json", json);
SyntaxHighlighter.registerLanguage("jsx", jsx);
SyntaxHighlighter.registerLanguage("markdown", markdown);
SyntaxHighlighter.registerLanguage("markup", markup);
SyntaxHighlighter.registerLanguage("sql", sql);
SyntaxHighlighter.registerLanguage("tsx", tsx);
SyntaxHighlighter.registerLanguage("typescript", typescript);
SyntaxHighlighter.registerLanguage("yaml", yaml);

const languageAliases: Record<string, CodeLanguage> = {
  bash: { extension: "sh", label: "Bash", syntax: "bash" },
  css: { extension: "css", label: "CSS", syntax: "css" },
  diff: { extension: "diff", label: "Diff", syntax: "diff" },
  dockerfile: {
    extension: "dockerfile",
    iconPath: "Dockerfile",
    label: "Dockerfile",
    syntax: "docker",
  },
  go: { extension: "go", label: "go", syntax: "go" },
  html: { extension: "html", label: "HTML", syntax: "markup" },
  javascript: { extension: "js", label: "JavaScript", syntax: "javascript" },
  js: { extension: "js", label: "JavaScript", syntax: "javascript" },
  json: { extension: "json", label: "JSON", syntax: "json" },
  jsx: { extension: "jsx", label: "JSX", syntax: "jsx" },
  markdown: { extension: "md", label: "Markdown", syntax: "markdown" },
  md: { extension: "md", label: "Markdown", syntax: "markdown" },
  sh: { extension: "sh", label: "Shell", syntax: "bash" },
  shell: { extension: "sh", label: "Shell", syntax: "bash" },
  sql: { extension: "sql", label: "SQL", syntax: "sql" },
  ts: { extension: "ts", label: "TypeScript", syntax: "typescript" },
  tsx: { extension: "tsx", label: "TSX", syntax: "tsx" },
  typescript: { extension: "ts", label: "TypeScript", syntax: "typescript" },
  yaml: { extension: "yaml", label: "YAML", syntax: "yaml" },
  yml: { extension: "yaml", label: "YAML", syntax: "yaml" },
  zsh: { extension: "sh", label: "Zsh", syntax: "bash" },
};

export function Markdown({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  return (
    <div className={cn("pp-markdown", className)}>
      <ReactMarkdown
        components={{
          a: ({ children, href }) => (
            <a href={href} rel="noreferrer" target="_blank">
              {children}
            </a>
          ),
          pre: ({ children }) => {
            const codeElement = codeElementFromPre(children);
            if (!codeElement) {
              return <pre>{children}</pre>;
            }

            const language = languageFromClassName(codeElement.className);
            const content = codeContent(codeElement.children);
            if (!language) {
              return (
                <pre>
                  <code>{content}</code>
                </pre>
              );
            }

            return <CodeBlock code={content} language={language} />;
          },
        }}
        remarkPlugins={[remarkGfm]}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}

interface CodeLanguage {
  extension: string;
  iconPath?: string;
  label: string;
  syntax?: string;
}

function CodeBlock({
  code,
  language,
}: {
  code: string;
  language: CodeLanguage;
}) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">(
    "idle",
  );
  const buttonLabel =
    copyState === "copied"
      ? "Copied code"
      : copyState === "failed"
        ? "Copy failed"
        : "Copy code";
  const languageIconPath = useMemo(
    () => language.iconPath ?? `code.${language.extension}`,
    [language.extension, language.iconPath],
  );

  useEffect(() => {
    if (copyState === "idle") {
      return;
    }

    const timeout = window.setTimeout(() => setCopyState("idle"), 1800);
    return () => window.clearTimeout(timeout);
  }, [copyState]);

  const handleCopy = async () => {
    try {
      if (!navigator.clipboard?.writeText) {
        setCopyState("failed");
        return;
      }

      await navigator.clipboard.writeText(code);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
    }
  };

  return (
    <figure className="pp-code-block">
      <figcaption className="pp-code-block__header">
        <span className="pp-code-block__language">
          <FileIcon className="size-3.5" path={languageIconPath} />
          {language.label}
        </span>
        <button
          aria-label={buttonLabel}
          className="pp-code-block__copy"
          onClick={() => void handleCopy()}
          title={buttonLabel}
          type="button"
        >
          {copyState === "copied" ? <Check /> : <Copy />}
        </button>
      </figcaption>
      <SyntaxHighlighter
        CodeTag="code"
        PreTag="pre"
        language={language.syntax}
        style={{}}
        useInlineStyles={false}
        wrapLongLines
      >
        {code}
      </SyntaxHighlighter>
    </figure>
  );
}

function codeContent(children: ReactNode) {
  return textFromNode(children).replace(/\n$/, "");
}

function codeElementFromPre(children: ReactNode) {
  const child = Children.toArray(children)[0];
  if (!isValidElement(child) || child.type !== "code") {
    return undefined;
  }

  const props = child.props as { children?: ReactNode; className?: string };
  return props;
}

function textFromNode(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return `${node}`;
  }

  if (isValidElement(node)) {
    const props = node.props as { children?: ReactNode };
    return textFromNode(props.children);
  }

  const children = Children.toArray(node);
  if (children.length > 0) {
    return children.map((child) => textFromNode(child)).join("");
  }

  return "";
}

function languageFromClassName(className?: string) {
  const rawLanguage = className?.match(/language-([^\s]+)/)?.[1];
  if (!rawLanguage) {
    return undefined;
  }

  const normalizedLanguage = rawLanguage.toLowerCase();
  return (
    languageAliases[normalizedLanguage] ?? {
      extension: normalizedLanguage,
      label: normalizedLanguage,
    }
  );
}
