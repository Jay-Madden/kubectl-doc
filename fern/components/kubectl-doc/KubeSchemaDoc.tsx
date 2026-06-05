"use client";

import { useEffect, useRef } from "react";

import { kubectlDocStyles } from "./kubectl-doc-styles";

export type KubeSchemaLine = {
  index: number;
  text: string;
  depth: number;
  field?: string;
  path?: string;
  code?: boolean;
  metadata?: boolean;
  required?: boolean;
  foldable?: boolean;
  collapsed?: boolean;
  detailId?: string;
};

export type KubeSchemaField = {
  id: string;
  path: string;
  type: string;
  required: boolean;
  description?: string;
  metadata?: string[];
};

export type KubeSchemaDocument = {
  apiVersion: string;
  group: string;
  version: string;
  kind: string;
  resource?: string;
  complete?: boolean;
  fullPayloadURL?: string;
  lines: KubeSchemaLine[];
  fields: KubeSchemaField[];
};

type KubectlDocController = {
  destroy: () => void;
  focusPath?: (path: string, options?: { scroll?: boolean }) => boolean;
  expandPath?: (path: string) => boolean;
  collapsePath?: (path: string) => boolean;
  setFilter?: (filter: string) => void;
  clearFilter?: () => void;
  snapshot?: () => { currentPath: string; filter: string };
};

type KubectlDocRuntime = {
  mount: (
    root: HTMLElement,
    options: {
      initialSchema: KubeSchemaDocument;
      filtering: boolean;
      detailsMode?: "inline-side" | "side-overlay";
      loadFullSchema?: () => Promise<KubeSchemaDocument> | KubeSchemaDocument | false | void;
    },
  ) => KubectlDocController;
};

declare global {
  interface Window {
    KubectlDoc?: KubectlDocRuntime;
  }
}

type KubeSchemaDocProps = {
  data: KubeSchemaDocument;
  filtering?: boolean;
  loadFullSchema?: () => Promise<KubeSchemaDocument> | KubeSchemaDocument | false | void;
  onLoadFull?: () => Promise<KubeSchemaDocument> | KubeSchemaDocument | false | void;
};

let runtimePromise: Promise<KubectlDocRuntime> | null = null;
const styleElementID = "kubectl-doc-fern-styles";

function ensureKubectlDocStyles() {
  if (typeof document === "undefined" || document.getElementById(styleElementID)) {
    return;
  }

  const style = document.createElement("style");
  style.id = styleElementID;
  style.textContent = kubectlDocStyles;
  document.head.appendChild(style);
}

function ensureKubectlDocRuntime() {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("kubectl-doc runtime is only available in the browser"));
  }
  if (window.KubectlDoc) {
    return Promise.resolve(window.KubectlDoc);
  }
  if (!runtimePromise) {
    runtimePromise = import("./kubectl-doc-runtime.js").then(() => {
      if (!window.KubectlDoc) {
        throw new Error("kubectl-doc runtime did not register window.KubectlDoc");
      }
      return window.KubectlDoc;
    });
  }
  return runtimePromise;
}

function resolveSchemaSource(source: string) {
  if (source.startsWith("http://") || source.startsWith("https://") || source.startsWith("/")) {
    return source;
  }
  return new URL(source, window.location.href.replace(/\/$/, "")).toString();
}

function decodeBase64UTF8(value: string) {
  const bytes = Uint8Array.from(atob(value.replace(/\s+/g, "")), (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function parseSchemaPayload(payload: string): KubeSchemaDocument {
  const encodedMatch = payload.match(/```kubectl-doc-schema\s*([\s\S]*?)\s*```/);
  if (encodedMatch) {
    return JSON.parse(decodeBase64UTF8(encodedMatch[1])) as KubeSchemaDocument;
  }

  const jsonMatch = payload.match(/```json\s*([\s\S]*?)\s*```/);
  return JSON.parse(jsonMatch ? jsonMatch[1] : payload) as KubeSchemaDocument;
}

function defaultLoadFullSchema(data: KubeSchemaDocument) {
  if (data.complete || !data.fullPayloadURL) {
    return undefined;
  }

  return () =>
    fetch(resolveSchemaSource(data.fullPayloadURL as string))
      .then((response) => {
        if (!response.ok) {
          throw new Error(`${response.status} ${response.statusText}`);
        }
        return response.text();
      })
      .then(parseSchemaPayload);
}

export function KubeSchemaDoc({ data, filtering = true, loadFullSchema, onLoadFull }: KubeSchemaDocProps) {
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    let controller: KubectlDocController | undefined;

    ensureKubectlDocStyles();
    ensureKubectlDocRuntime()
      .then((runtime) => {
        if (cancelled || !rootRef.current) {
          return;
        }
        controller = runtime.mount(rootRef.current, {
          initialSchema: data,
          filtering,
          detailsMode: "side-overlay",
          loadFullSchema: loadFullSchema ?? onLoadFull ?? defaultLoadFullSchema(data),
        });
      })
      .catch((error: unknown) => {
        console.error("kubectl-doc runtime failed to mount", error);
      });

    return () => {
      cancelled = true;
      controller?.destroy();
    };
  }, [data, filtering, loadFullSchema, onLoadFull]);

  return <div ref={rootRef} className="kubectl-doc kdoc-fern-host" />;
}

export default KubeSchemaDoc;
