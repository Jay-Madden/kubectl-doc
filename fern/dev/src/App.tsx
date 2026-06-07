import { useCallback, useEffect, useState } from "react";

import { KubeSchemaDoc, type KubeSchemaDocument } from "../../../react/kubectl-doc/KubeSchemaDoc";

import "./fern-preview.css";

type SchemaReference = {
  label: string;
  data: KubeSchemaDocument;
};

type SchemaManifest = {
  title: string;
  schemas: SchemaReference[];
};

function resolveSchemaSource(source: string) {
  if (source.startsWith("http://") || source.startsWith("https://") || source.startsWith("/")) {
    return source;
  }
  return new URL(source, window.location.href.replace(/\/$/, "")).toString();
}

function StatefulFullLoadSchemaDoc({ data, filtering = true }: { data: KubeSchemaDocument; filtering?: boolean }) {
  const [activeData, setActiveData] = useState(data);

  useEffect(() => {
    setActiveData(data);
  }, [data]);

  const loadFullSchema = useCallback(() => {
    if (activeData.complete || !activeData.fullPayloadURL) {
      return false;
    }

    return fetch(resolveSchemaSource(activeData.fullPayloadURL))
      .then((response) => {
        if (!response.ok) {
          throw new Error(`${response.status} ${response.statusText}`);
        }
        return response.json() as Promise<KubeSchemaDocument>;
      })
      .then((next) => {
        setActiveData(next);
        return next;
      });
  }, [activeData]);

  return <KubeSchemaDoc data={activeData} filtering={filtering} loadFullSchema={loadFullSchema} />;
}

export function App() {
  const [manifest, setManifest] = useState<SchemaManifest | null>(null);
  const [active, setActive] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch("/schemas/manifest.json")
      .then((response) => {
        if (!response.ok) {
          throw new Error(`${response.status} ${response.statusText}`);
        }
        return response.json() as Promise<SchemaManifest>;
      })
      .then((next) => {
        if (!cancelled) {
          setManifest(next);
          setActive(0);
        }
      })
      .catch((loadError: unknown) => {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : String(loadError));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return <main className="fern-dev-page">Schema fixture failed to load: {error}</main>;
  }
  if (!manifest) {
    return <main className="fern-dev-page">Loading schema fixture...</main>;
  }

  const schema = manifest.schemas[active];
  const query = new URLSearchParams(window.location.search);
  const statefulFullLoad = query.has("statefulFullLoad");
  const filtering = !query.has("disableFiltering");

  return (
    <main className="fern-dev-page">
      <nav className="fern-dev-breadcrumb" aria-label="Breadcrumb">
        <span>Kubernetes Deployment</span>
        <span aria-hidden="true">›</span>
        <span>API Reference</span>
      </nav>
      <h1>{manifest.title}</h1>
      <div className="fern-dev-actions" aria-hidden="true">
        <span>Copy page</span>
        <span>View as Markdown</span>
        <span>Open in Claude</span>
        <span>More actions</span>
      </div>

      <div className="fern-dev-tabs" role="tablist" aria-label="API versions">
        {manifest.schemas.map((item, index) => (
          <button
            key={item.label}
            type="button"
            role="tab"
            aria-selected={index === active}
            className={index === active ? "active" : ""}
            onClick={() => setActive(index)}
          >
            {item.label}
          </button>
        ))}
      </div>

      <section className="fern-dev-card" aria-label={`${schema.label} schema`}>
        {statefulFullLoad ? (
          <StatefulFullLoadSchemaDoc key={schema.label} data={schema.data} filtering={filtering} />
        ) : (
          <KubeSchemaDoc key={schema.label} data={schema.data} filtering={filtering} />
        )}
      </section>
    </main>
  );
}
