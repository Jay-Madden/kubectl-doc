import { expect, type Locator, type Page, test } from "@playwright/test";

const performanceBudgetMs = 200;
const filterKeystrokeBudgetMs = 50;

type KubeDocPerfEntry = {
  name: string;
  duration: number;
  detail?: Record<string, number | boolean | string>;
};

async function mountedHost(page: Page): Promise<Locator> {
  const host = page.locator(".kdoc-react-host").first();
  await page.waitForFunction(() => {
    const node = document.querySelector(".kdoc-react-host") as HTMLElement & {
      __kubectlDocController?: unknown;
    };
    return Boolean(node?.__kubectlDocController);
  });
  return host;
}

async function mountedDomHost(page: Page, selector = "[data-kubectl-doc]"): Promise<Locator> {
  const host = page.locator(selector).first();
  await page.waitForFunction((hostSelector) => {
    const node = document.querySelector(hostSelector) as HTMLElement & {
      __kubectlDocController?: unknown;
    };
    return Boolean(node?.__kubectlDocController);
  }, selector);
  return host;
}

async function latestPerfEntry(page: Page, name: string): Promise<KubeDocPerfEntry | undefined> {
  return page.evaluate((entryName) => {
    const entries = ((window as unknown as { __kubectlDocPerf?: KubeDocPerfEntry[] }).__kubectlDocPerf ?? []).filter(
      (entry) => entry.name === entryName,
    );
    return entries.at(-1);
  }, name);
}

async function waitForPerfEntry(page: Page, name: string): Promise<KubeDocPerfEntry> {
  await expect
    .poll(() => latestPerfEntry(page, name), { timeout: 10_000 })
    .toEqual(expect.objectContaining({ name }));
  const entry = await latestPerfEntry(page, name);
  if (!entry) {
    throw new Error(`missing perf entry ${name}`);
  }
  return entry;
}

async function visibleSchemaLineCount(host: Locator): Promise<number> {
  return host.evaluate((node) => {
    return Array.from(node.querySelectorAll<HTMLElement>("[data-kdoc-line]")).filter((line) => {
      const style = getComputedStyle(line);
      const box = line.getBoundingClientRect();
      return style.display !== "none" && !line.hidden && box.width > 0 && box.height > 0;
    }).length;
  });
}

async function selectableTreeText(host: Locator): Promise<string> {
  return host.locator(".kdoc-tree").first().evaluate((tree) => {
    const range = document.createRange();
    const selection = window.getSelection();
    range.selectNodeContents(tree);
    selection?.removeAllRanges();
    selection?.addRange(range);
    const text = selection?.toString() ?? "";
    selection?.removeAllRanges();
    return text;
  });
}

async function selectedFieldPath(host: Locator): Promise<string> {
  return host.evaluate((node) => {
    return node.querySelector<HTMLElement>("[data-kdoc-field].kdoc-selected")?.getAttribute("data-path") ?? "";
  });
}

function expectCopyableDynamoYAML(text: string): void {
  expect(text).toContain("apiVersion: nvidia.com/v1beta1");
  expect(text).toContain("kind: DynamoGraphDeployment");
  expect(text).not.toMatch(/[▶▼]/);
  expect(text).not.toContain("toggle");
}

function perfNumber(entry: KubeDocPerfEntry, key: string): number {
  const value = entry.detail?.[key];
  if (typeof value !== "number") {
    throw new Error(`missing numeric perf detail ${entry.name}.${key}`);
  }
  return value;
}

test("renders YAML token markup as DOM nodes", async ({ page }) => {
  await page.goto("/");

  const tree = page.locator(".kdoc-tree").first();
  await expect(tree).toContainText("apiVersion: nvidia.com/v1beta1");
  await expect(tree).not.toContainText("kdoc-yaml-key");
  await expect(tree.locator('[data-kdoc-field][data-path="apiVersion"] .kdoc-yaml-key')).toHaveCount(1);
  await expect(tree.locator(".kdoc-yaml-punct").first()).toBeVisible();
});

test("mounts the initial Fern payload under the interactive budget", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  const mount = await waitForPerfEntry(page, "mount");
  const lines = perfNumber(mount, "lines");
  expect(mount.duration).toBeLessThanOrEqual(performanceBudgetMs);
  expect(lines).toBeLessThan(1_000);
  await expect(host.locator("[data-kdoc-line]")).toHaveCount(lines);
});

test("mounts the standalone runtime under the interactive budget", async ({ page }) => {
  await page.goto("/");
  await mountedHost(page);

  const mount = await page.evaluate(async () => {
    const manifest = (await fetch("/schemas/manifest.json").then((response) => response.json())) as {
      schemas: Array<{ data: unknown }>;
    };
    document.body.innerHTML = '<main><div id="standalone-schema"></div></main>';
    (window as unknown as { __kubectlDocPerf?: KubeDocPerfEntry[] }).__kubectlDocPerf = [];
    const root = document.getElementById("standalone-schema");
    if (!root || !window.KubectlDoc) {
      throw new Error("missing standalone runtime");
    }
    window.KubectlDoc.mount(root, {
      initialSchema: manifest.schemas[0].data as Parameters<typeof window.KubectlDoc.mount>[1]["initialSchema"],
      filtering: true,
      detailsMode: "side-overlay",
      wrapControl: false,
      wrapComments: true,
    });
    return (window as unknown as { __kubectlDocPerf?: KubeDocPerfEntry[] }).__kubectlDocPerf?.find(
      (entry) => entry.name === "mount",
    );
  });

  if (!mount) {
    throw new Error("missing standalone mount perf entry");
  }
  expect(mount.duration).toBeLessThanOrEqual(performanceBudgetMs);
  expect(perfNumber(mount, "lines")).toBeLessThan(1_000);
});

test("serves generated full schema sidecars as static JSON assets", async ({ page }) => {
  await page.goto("/");

  const sidecars = await page.evaluate(async () => {
    const manifestResponse = await fetch("/schemas/manifest.json");
    const manifest = (await manifestResponse.json()) as {
      schemas: Array<{ label: string; data: { fullPayloadURL?: string } }>;
    };

    return Promise.all(
      manifest.schemas.map(async (schema) => {
        if (!schema.data.fullPayloadURL) {
          throw new Error(`schema ${schema.label} does not expose fullPayloadURL`);
        }
        const response = await fetch(new URL(schema.data.fullPayloadURL, window.location.href).toString());
        const body = await response.text();
        const payload = JSON.parse(body) as { complete?: boolean; lines?: unknown[]; fields?: unknown[] };
        return {
          label: schema.label,
          url: response.url,
          status: response.status,
          contentType: response.headers.get("content-type") ?? "",
          size: new TextEncoder().encode(body).length,
          complete: payload.complete === true,
          lines: payload.lines?.length ?? 0,
          fields: payload.fields?.length ?? 0,
          firstByte: body.trimStart().slice(0, 1),
        };
      }),
    );
  });

  expect(sidecars.length).toBeGreaterThanOrEqual(2);
  for (const sidecar of sidecars) {
    expect(sidecar.status, sidecar.label).toBe(200);
    expect(sidecar.url, sidecar.label).toMatch(/-full\.json$/);
    expect(sidecar.url, sidecar.label).not.toMatch(/\.md($|[?#])/);
    expect(sidecar.contentType, sidecar.label).toContain("application/json");
    expect(sidecar.firstByte, sidecar.label).toBe("{");
    expect(sidecar.complete, sidecar.label).toBeTruthy();
    expect(sidecar.lines, sidecar.label).toBeGreaterThan(5_000);
    expect(sidecar.fields, sidecar.label).toBeGreaterThan(1_000);
    expect(sidecar.size, sidecar.label).toBeGreaterThan(2_000_000);
  }
});

test("measures deterministic schema size tiers against shared runtime budgets", async ({ page }) => {
  test.slow();
  await page.goto("/");
  await mountedHost(page);

  const tiers = [
    { name: "small", fields: 90, minBytes: 100_000, maxBytes: 150_000 },
    { name: "medium", fields: 500, minBytes: 500_000, maxBytes: 650_000 },
    { name: "large", fields: 2_000, minBytes: 2_000_000, maxBytes: 2_400_000 },
  ];
  const results = await page.evaluate(async ({ testTiers }) => {
    document.querySelectorAll<HTMLElement>(".kdoc-react-host,[data-kubectl-doc]").forEach((node) => {
      (
        node as HTMLElement & {
          __kubectlDocController?: { destroy?: () => void };
        }
      ).__kubectlDocController?.destroy?.();
    });
    document.body.innerHTML = "";

    function perfEntries(name: string) {
      return ((window as unknown as { __kubectlDocPerf?: KubeDocPerfEntry[] }).__kubectlDocPerf ?? []).filter(
        (entry) => entry.name === name,
      );
    }

    function latestPerf(name: string) {
      const entries = perfEntries(name);
      const entry = entries.at(-1);
      if (!entry) {
        throw new Error(`missing perf entry ${name}`);
      }
      return entry;
    }

    async function waitForPerf(name: string) {
      for (let i = 0; i < 100; i++) {
        const entries = perfEntries(name);
        if (entries.length > 0) {
          return entries.at(-1)!;
        }
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      throw new Error(`timed out waiting for ${name}`);
    }

    function scalarLine(index: number, depth: number, field: string, path: string, value: string) {
      return {
        index,
        depth,
        field,
        path,
        code: true,
        detailId: `field-${path.toLowerCase().replaceAll(".", "-")}`,
        tokens: [
          { t: "  ".repeat(depth) },
          { k: "key", t: field },
          { k: "punct", t: ":" },
          { t: " " },
          { k: "scalar", t: value },
        ],
      };
    }

    function buildPayload(fieldCount: number) {
      const lines = [
        scalarLine(0, 0, "apiVersion", "apiVersion", "example.io/v1"),
        scalarLine(1, 0, "kind", "kind", "PerfWidget"),
        {
          index: 2,
          depth: 0,
          field: "spec",
          path: "spec",
          code: true,
          required: true,
          foldable: true,
          collapsed: true,
          detailId: "field-spec",
          tokens: [
            { k: "key", t: "spec" },
            { k: "punct", t: ":" },
            { t: " " },
            { k: "comment", t: "# required" },
          ],
        },
      ];
      const fields = [
        {
          id: "field-apiversion",
          path: "apiVersion",
          type: "string",
          required: true,
          description: "API version.",
        },
        { id: "field-kind", path: "kind", type: "string", required: true, description: "Kind." },
        { id: "field-spec", path: "spec", type: "object", required: true, description: "Spec." },
      ];

      for (let i = 0; i < fieldCount; i++) {
        const suffix = String(i).padStart(5, "0");
        const field = `field${suffix}`;
        const path = `spec.${field}`;
        const id = `field-spec-${field}`;
        const needle = `needle-${suffix}`;
        const description = `${needle} deterministic performance fixture description for ${field} `.repeat(4).trim();
        lines.push({
          index: lines.length,
          depth: 1,
          path,
          detailId: id,
          commentGroup: `description-${i}`,
          comment: {
            prefix: "  # ",
            wrapPrefix: "  # ",
            text: description,
          },
        });
        lines.push(scalarLine(lines.length, 1, field, path, "\"<string>\""));
        fields.push({
          id,
          path,
          type: "string",
          required: false,
          description,
        });
      }

      return {
        apiVersion: "example.io/v1",
        group: "example.io",
        version: "v1",
        kind: "PerfWidget",
        resource: "perfwidgets",
        complete: true,
        lines,
        fields,
      };
    }

    function shallowPayload(full: ReturnType<typeof buildPayload>) {
      return {
        ...full,
        complete: false,
        fullPayloadURL: "./synthetic-full.json",
        lines: full.lines.slice(0, 3),
        fields: full.fields.slice(0, 3),
      };
    }

    const measurements = [];
    for (const tier of testTiers) {
      const full = buildPayload(tier.fields);
      const shallow = shallowPayload(full);
      const fullBytes = new TextEncoder().encode(JSON.stringify(full)).length;
      const root = document.createElement("div");
      root.className = "kubectl-doc kdoc-embedded-host kdoc-react-host";
      document.body.appendChild(root);
      (window as unknown as { __kubectlDocPerf?: KubeDocPerfEntry[] }).__kubectlDocPerf = [];

      const runtime = window.KubectlDoc;
      if (!runtime) {
        throw new Error("missing kubectl-doc runtime");
      }
      const controller = runtime.mount(root, {
        initialSchema: shallow,
        filtering: true,
        detailsMode: "side-overlay",
        wrapControl: false,
        wrapComments: true,
        loadFullSchema: () => Promise.resolve(full),
      });
      const mount = latestPerf("mount");
      root.focus();
      const activation = await waitForPerf("full-schema-activate");
      controller.setFilter?.(`needle-${String(tier.fields - 1).padStart(5, "0")}`);
      const exactFilter = latestPerf("filter-apply");
      const exactProjection = latestPerf("projection-render");
      const exactDomLines = root.querySelectorAll("[data-kdoc-line]").length;
      const exactVisibleLines = Array.from(root.querySelectorAll<HTMLElement>("[data-kdoc-line]")).filter((line) => {
        const style = getComputedStyle(line);
        const box = line.getBoundingClientRect();
        return style.display !== "none" && !line.hidden && box.width > 0 && box.height > 0;
      }).length;
      controller.setFilter?.("needle-000");
      const prefixFilter = latestPerf("filter-apply");
      const prefixProjection = latestPerf("projection-render");

      measurements.push({
        name: tier.name,
        fullBytes,
        fullLines: full.lines.length,
        fullFields: full.fields.length,
        exactDomLines,
        exactVisibleLines,
        prefixDomLines: root.querySelectorAll("[data-kdoc-line]").length,
        mount,
        activation,
        exactFilter,
        exactProjection,
        prefixFilter,
        prefixProjection,
      });

      controller.destroy();
      root.remove();
    }
    return measurements;
  }, { testTiers: tiers });

  expect(results).toHaveLength(tiers.length);
  for (const result of results) {
    const tier = tiers.find((item) => item.name === result.name);
    if (!tier) {
      throw new Error(`unexpected tier ${result.name}`);
    }
    expect(result.fullBytes, result.name).toBeGreaterThanOrEqual(tier.minBytes);
    expect(result.fullBytes, result.name).toBeLessThanOrEqual(tier.maxBytes);
    expect(result.mount.duration, `${result.name} mount`).toBeLessThanOrEqual(performanceBudgetMs);
    expect(result.activation.duration, `${result.name} activation`).toBeLessThanOrEqual(performanceBudgetMs);
    expect(result.exactProjection.duration, `${result.name} exact projection`).toBeLessThanOrEqual(performanceBudgetMs);
    expect(result.prefixProjection.duration, `${result.name} prefix projection`).toBeLessThanOrEqual(performanceBudgetMs);
    expect(result.exactFilter.duration, `${result.name} exact filter`).toBeLessThanOrEqual(filterKeystrokeBudgetMs);
    expect(result.prefixFilter.duration, `${result.name} prefix filter`).toBeLessThanOrEqual(filterKeystrokeBudgetMs);
    expect(result.mount.detail?.lines, `${result.name} shallow lines`).toBe(3);
    expect(result.activation.detail?.lines, `${result.name} full lines`).toBe(result.fullLines);
    expect(result.activation.detail?.renderedLines, `${result.name} rendered lines after activation`).toBe(3);
    expect(result.exactFilter.detail?.renderedFull, `${result.name} rendered full after filter`).toBeTruthy();
    expect(result.exactDomLines, `${result.name} exact projected DOM lines`).toBeLessThan(result.fullLines / 2);
    expect(result.exactVisibleLines, `${result.name} exact visible projected lines`).toBeLessThan(result.fullLines / 2);
    expect(result.prefixDomLines, `${result.name} prefix projected DOM lines`).toBeLessThanOrEqual(result.fullLines);
  }
});

test("filters DOM-mounted standalone HTML without clearing the YAML tree", async ({ page }) => {
  await page.goto("/");
  const host = await mountedHost(page);
  await host.evaluate((node) => {
    (node as HTMLElement & { __kubectlDocController?: { destroy?: () => void } }).__kubectlDocController?.destroy?.();
  });

  const staticRoot = await page.evaluateHandle(() => {
    document.body.innerHTML = `
      <main>
        <div id="static-schema" class="kubectl-doc" data-kubectl-doc>
          <div class="kdoc-layout">
            <section class="kdoc-docs">
              <div class="kdoc-filter-overlay" data-kdoc-filter-overlay hidden></div>
              <section class="kdoc-version">
                <div class="kdoc-tree" role="tree" aria-label="Widget YAML schema">
                  <div class="kdoc-line" role="treeitem" data-kdoc-line data-kdoc-field data-kdoc-field-name="apiVersion" data-kdoc-filter-text="apiVersion" data-index="0" data-depth="0" data-path="apiVersion" data-detail-id="field-apiversion" data-detail-html="">
                    <span class="kdoc-gutter"></span><span class="kdoc-yaml-text"><span class="kdoc-yaml-key">apiVersion</span><span class="kdoc-yaml-punct">:</span> example.io/v1</span>
                  </div>
                  <div class="kdoc-line" role="treeitem" data-kdoc-line data-kdoc-field data-kdoc-field-name="spec" data-kdoc-filter-text="spec
Specification" data-index="1" data-depth="0" data-path="spec" data-detail-id="field-spec" data-detail-html="">
                    <span class="kdoc-gutter"></span><span class="kdoc-yaml-text"><span class="kdoc-yaml-key">spec</span><span class="kdoc-yaml-punct">:</span></span>
                  </div>
                </div>
              </section>
            </section>
            <aside class="kdoc-details"><div data-kdoc-detail-body></div></aside>
          </div>
        </div>
      </main>`;
    const root = document.getElementById("static-schema");
    if (!root || !window.KubectlDoc) {
      throw new Error("missing standalone runtime");
    }
    window.KubectlDoc.mount(root, { filtering: true } as Parameters<typeof window.KubectlDoc.mount>[1]);
    return root;
  });
  const staticHost = page.locator("#static-schema");

  await page.keyboard.type("spec");

  await expect(staticHost.locator(".kdoc-filter-overlay")).toContainText("spec");
  await expect(staticHost.locator('[data-kdoc-field][data-path="spec"]')).toBeVisible();
  await expect(staticHost.locator(".kdoc-tree [data-kdoc-line]")).toHaveCount(2);
  expect(await visibleSchemaLineCount(staticHost)).toBeGreaterThan(0);
  await staticRoot.dispose();
});

test("drives the browser overview fixture with keyboard and filter parity", async ({ page }) => {
  await page.goto("/fixtures/browser-overview.html");

  const overlay = page.locator("[data-kdoc-filter-overlay]");
  const selected = page.locator(".kdoc-overview-selected");
  await expect(selected).toHaveText("v1");

  await page.keyboard.type("dgd");
  await expect(overlay).toContainText("filter: dgd");
  await expect(page.locator('[data-resource-name="dynamographdeployments"]')).toBeVisible();
  await expect(page.locator('[data-resource-name="pods"]')).toBeHidden();
  await expect(page.locator('[data-resource-name="dynamographdeployments"] .kdoc-overview-selected')).toHaveCount(1);

  await page.keyboard.press("Escape");
  await expect(overlay).toBeHidden();
  await expect(page.locator('[data-resource-name="pods"]')).toBeVisible();

  await page.keyboard.press("Home");
  await expect(page.locator(".kdoc-overview-selected")).toHaveAttribute("href", "/?resource=pods&version=v1");
  await page.keyboard.press("ArrowRight");
  await expect(page.locator(".kdoc-overview-selected")).toHaveAttribute(
    "href",
    "/?group=apps&resource=deployments&version=v1",
  );
  await page.keyboard.press("Tab");
  await expect(page.locator(".kdoc-overview-selected")).toHaveAttribute(
    "href",
    "/?group=nvidia.com&resource=dynamographdeployments&version=v1beta1",
  );

  await page.keyboard.press("/");
  await expect(overlay).toBeHidden();
});

test("filters and folds the browser-selected schema fixture without clearing the tree", async ({ page }) => {
  await page.goto("/fixtures/browser-schema.html");
  const host = await mountedDomHost(page);
  await expect(host).toHaveAttribute("data-kdoc-back-url", "/");

  const metadata = host.locator('[data-kdoc-field][data-path="metadata"]').first();
  const metadataName = host.locator('[data-kdoc-field][data-path="metadata.name"]').first();
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await metadata.locator("[data-kdoc-toggle]").click();
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(metadataName).toBeVisible();

  await host.click();
  await page.keyboard.type("secretKeyRef");
  await expect(host.locator(".kdoc-filter-overlay")).toContainText("secretKeyRef");
  await expect(
    host.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]').first(),
  ).toBeVisible({ timeout: 10_000 });
  expect(await visibleSchemaLineCount(host)).toBeGreaterThan(0);

  await page.keyboard.press("Escape");
  await expect(host.locator(".kdoc-filter-overlay")).toBeHidden();
  await page.keyboard.press("/");
  await expect(host.locator(".kdoc-filter-overlay")).toBeHidden();
});

test("drives browser-selected schema keyboard navigation", async ({ page }) => {
  await page.goto("/fixtures/browser-schema.html");
  const host = await mountedDomHost(page);
  const metadata = host.locator('[data-kdoc-field][data-path="metadata"]').first();
  const metadataName = host.locator('[data-kdoc-field][data-path="metadata.name"]').first();

  await expect.poll(() => selectedFieldPath(host)).toBe("apiVersion");
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => selectedFieldPath(host)).toBe("kind");
  await page.keyboard.press("Home");
  await expect.poll(() => selectedFieldPath(host)).toBe("apiVersion");

  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");

  await page.keyboard.press("ArrowRight");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");
  await expect(metadataName).toBeVisible();

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata.name");
  await expect(host.locator(".kdoc-details")).toContainText("metadata.name");

  await page.keyboard.press("ArrowLeft");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");
  await page.keyboard.press("ArrowLeft");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");

  await page.keyboard.press("Enter");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Home");
  await page.keyboard.press("Tab");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");
});

test("keeps root context visible when navigating to the first schema field", async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 650 });
  await page.goto("/fixtures/browser-schema.html");
  const host = await mountedDomHost(page);

  await page.keyboard.press("End");
  await expect.poll(() => selectedFieldPath(host)).not.toBe("apiVersion");
  await page.keyboard.press("Home");
  await expect.poll(() => selectedFieldPath(host)).toBe("apiVersion");

  const context = await page.evaluate(() => {
    const header = document.querySelector<HTMLElement>(".kdoc-header h1");
    const description = document.querySelector<HTMLElement>('[data-detail-id^="root-description"]');
    const apiVersion = document.querySelector<HTMLElement>('[data-kdoc-field][data-path="apiVersion"]');
    const visible = (element: HTMLElement | null) => {
      if (!element) {
        return false;
      }
      const rect = element.getBoundingClientRect();
      return rect.bottom > 0 && rect.top < window.innerHeight;
    };
    return {
      apiVersionVisible: visible(apiVersion),
      descriptionAboveAPI: Boolean(
        description &&
          apiVersion &&
          description.getBoundingClientRect().top < apiVersion.getBoundingClientRect().top,
      ),
      descriptionVisible: visible(description),
      headerVisible: visible(header),
    };
  });
  expect(context).toEqual({
    apiVersionVisible: true,
    descriptionAboveAPI: true,
    descriptionVisible: true,
    headerVisible: true,
  });
});

test("keeps selected YAML text free of fold gutters", async ({ page }) => {
  await page.goto("/fixtures/browser-schema.html");
  let host = await mountedDomHost(page);
  expectCopyableDynamoYAML(await selectableTreeText(host));

  await page.goto("/");
  host = await mountedHost(page);
  expectCopyableDynamoYAML(await selectableTreeText(host));
});

test("filters only the focused version in DOM-mounted multi-version HTML", async ({ page }) => {
  await page.goto("/fixtures/multiversion-schema.html");
  const host = await mountedDomHost(page);
  const versions = host.locator(".kdoc-version");
  await expect(versions).toHaveCount(2);

  const beta = versions.nth(0);
  const alpha = versions.nth(1);
  await expect(beta.locator("h2")).toContainText("nvidia.com/v1beta1");
  await expect(alpha.locator("h2")).toContainText("nvidia.com/v1alpha1");
  await expect(beta.locator('[data-kdoc-field][data-path="kind"]').first()).toBeVisible();
  await expect(alpha.locator('[data-kdoc-field][data-path="kind"]').first()).toBeVisible();

  await beta.locator('[data-kdoc-field][data-path="spec.components"]').first().evaluate((line) => {
    (line as HTMLElement).click();
  });
  await host.evaluate((node) => {
    (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } }).__kubectlDocController
      ?.setFilter("secretKeyRef");
  });

  await expect(host.locator(".kdoc-filter-overlay")).toContainText("secretKeyRef");
  await expect(beta).toHaveClass(/kdoc-filtering/);
  await expect(alpha).not.toHaveClass(/kdoc-filtering/);
  await expect(beta.locator('[data-kdoc-field][data-path="kind"]').first()).toBeHidden();
  await expect(alpha.locator('[data-kdoc-field][data-path="kind"]').first()).toBeVisible();
  await expect(
    beta.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]').first(),
  ).toBeVisible({ timeout: 10_000 });

  await host.evaluate((node) => {
    (node as HTMLElement & { __kubectlDocController?: { clearFilter: () => void } }).__kubectlDocController
      ?.clearFilter();
  });
  await expect(host.locator(".kdoc-filter-overlay")).toBeHidden();
  await expect(beta).not.toHaveClass(/kdoc-filtering/);
  await expect(beta.locator('[data-kdoc-field][data-path="kind"]').first()).toBeVisible();

  await alpha.locator('[data-kdoc-field][data-path="apiVersion"]').first().evaluate((line) => {
    (line as HTMLElement).click();
  });
  await host.evaluate((node) => {
    (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } }).__kubectlDocController
      ?.setFilter("secretKeyRef");
  });
  await expect(alpha).toHaveClass(/kdoc-filtering/);
  await expect(beta).not.toHaveClass(/kdoc-filtering/);
  await expect(alpha.locator('[data-kdoc-field][data-path="kind"]').first()).toBeHidden();
  await expect(beta.locator('[data-kdoc-field][data-path="kind"]').first()).toBeVisible();
});

test("keeps MkDocs-style embedded schemas on the shared overlay and wrapping contract", async ({ page }) => {
  await page.setViewportSize({ width: 1120, height: 900 });
  await page.goto("/fixtures/mkdocs-embedded-schema.html");

  const host = await mountedDomHost(page, ".kdoc-mkdocs-content [data-kubectl-doc]");
  await expect(host).toHaveAttribute("data-kdoc-details-mode", "side-overlay");
  await expect(host).toHaveAttribute("data-kdoc-auto-focus", "true");
  await expect(host).toHaveClass(/kdoc-details-side-overlay/);
  await expect(host.locator(".kdoc-view-controls")).toBeHidden();

  const details = host.locator(".kdoc-details");
  await expect(details).toBeVisible();
  await expect(details).toContainText("apiVersion");

  const components = host.locator('[data-kdoc-field][data-path="spec.components"]').first();
  await components.click();
  await expect(details).toHaveCSS("position", "fixed");
  await expect(details).toHaveCSS("z-index", "2147483647");
  await expect(details).toContainText("spec.components");

  const before = await details.boundingBox();
  await page.evaluate(() => window.scrollTo(0, 800));
  const after = await details.boundingBox();
  expect(before?.y).toBeDefined();
  expect(after?.y).toBeDefined();
  expect(Math.abs((after?.y ?? 0) - (before?.y ?? 0))).toBeLessThanOrEqual(1);

  await expect.poll(() =>
    page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth),
  ).toBeLessThanOrEqual(2);

  const wrappedComment = await host.locator("[data-kdoc-comment]").first().evaluate((comment) => ({
    separatorTextNodes: Array.from(comment.childNodes).filter(
      (node) => node.nodeType === Node.TEXT_NODE && /\S|\n/.test(node.textContent ?? ""),
    ).length,
    lines: Array.from(comment.querySelectorAll(".kdoc-comment-line")).map((line) => line.textContent ?? ""),
  }));
  expect(wrappedComment.separatorTextNodes).toBe(0);
  expect(wrappedComment.lines.length).toBeGreaterThan(1);
  expect(wrappedComment.lines.every((line) => line.trimStart().startsWith("#"))).toBeTruthy();
});

test("drives MkDocs-style embedded schema filtering and keyboard navigation", async ({ page }) => {
  await page.goto("/fixtures/mkdocs-embedded-schema.html");
  const host = await mountedDomHost(page, ".kdoc-mkdocs-content [data-kubectl-doc]");
  const metadata = host.locator('[data-kdoc-field][data-path="metadata"]').first();
  const annotations = host.locator('[data-kdoc-field][data-path="metadata.annotations"]').first();

  await host.locator('[data-kdoc-field][data-path="apiVersion"]').first().click();
  await expect.poll(() => selectedFieldPath(host)).toBe("apiVersion");
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => selectedFieldPath(host)).toBe("kind");
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata");

  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await page.keyboard.press("ArrowRight");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata.name");

  await page.keyboard.type("annotations");
  await expect(host.locator(".kdoc-filter-overlay")).toContainText("annotations");
  await expect(annotations).toBeVisible();
  await page.keyboard.press("Tab");
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata.annotations");

  await page.keyboard.press("Escape");
  await expect(host.locator(".kdoc-filter-overlay")).toBeHidden();
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
});

test("keeps Fern comments wrapped without exposing a wrap toggle", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  await expect(host).toHaveClass(/kdoc-has-focus/);
  await expect(host.locator(".kdoc-details")).toBeVisible();
  await expect(host.locator(".kdoc-details")).toContainText("apiVersion");
  await expect(host).toHaveClass(/kdoc-wrap-comments/);
  await expect(page.locator(".kdoc-view-controls")).toHaveCount(0);
  await expect(page.locator(".kdoc-wrap-toggle")).toHaveCount(0);

  const commentPrefix = page.locator(".kdoc-comment-prefix").first();
  const commentBody = page.locator(".kdoc-comment-body").first();
  await expect(commentPrefix).toHaveCSS("white-space", "pre");
  await expect(commentPrefix).toHaveCSS("overflow-wrap", "normal");
  await expect(commentBody).toHaveCSS("white-space", "pre");
  await expect(commentBody).toHaveCSS("overflow-wrap", "normal");
});

test("keeps Fern fold and details usable when filtering is disabled", async ({ page }) => {
  await page.goto("/?disableFiltering=1");

  const host = await mountedHost(page);
  await expect(host).toHaveClass(/kdoc-filter-disabled/);

  const visibleBeforeTyping = await visibleSchemaLineCount(host);
  await host.locator('[data-kdoc-field][data-path="apiVersion"]').first().click();
  await page.keyboard.type("annotations");
  await expect(host.locator(".kdoc-filter-overlay")).toBeHidden();
  await expect(host.locator(".kdoc-version")).not.toHaveClass(/kdoc-filtering/);
  expect(await visibleSchemaLineCount(host)).toBe(visibleBeforeTyping);
  await expect.poll(() => selectedFieldPath(host)).toBe("apiVersion");

  await page.keyboard.press("ArrowDown");
  await expect.poll(() => selectedFieldPath(host)).toBe("kind");

  const metadata = host.locator('[data-kdoc-field][data-path="metadata"]').first();
  const metadataName = host.locator('[data-kdoc-field][data-path="metadata.name"]').first();
  await metadata.click();
  await expect(host.locator(".kdoc-details")).toContainText("metadata");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await page.keyboard.press("Enter");
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(metadataName).toBeVisible();
});

test("semantically wraps single long comments with comment prefixes", async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 900 });
  await page.goto("/");
  await mountedHost(page);

  const wrappedComment = await page.evaluate(() => {
    const root = document.createElement("div");
    root.className = "kubectl-doc kdoc-embedded-host kdoc-react-host";
    root.style.width = "360px";
    document.body.appendChild(root);
    window.KubectlDoc?.mount(root, {
      filtering: true,
      wrapControl: false,
      wrapComments: true,
      initialSchema: {
        apiVersion: "example.io/v1",
        group: "example.io",
        version: "v1",
        kind: "Widget",
        complete: true,
        lines: [
          {
            index: 0,
            depth: 1,
            path: "spec.experimental",
            detailId: "field-experimental",
            commentGroup: "description-0",
            comment: {
              prefix: "    # ",
              wrapPrefix: "    # ",
              text:
                "experimental groups opt-in preview features whose API shape and behavior may change in breaking ways between v1beta1 releases, including disappearing without a name-preserving graduation path.",
            },
          },
          {
            index: 1,
            depth: 1,
            field: "experimental",
            path: "spec.experimental",
            detailId: "field-experimental",
            tokens: [
              { t: "  " },
              { k: "key", t: "experimental" },
              { k: "punct", t: ":" },
            ],
          },
        ],
        fields: [
          {
            id: "field-experimental",
            path: "spec.experimental",
            type: "object",
            required: false,
            description: "experimental groups opt-in preview features.",
          },
        ],
      },
    });

    const comment = root.querySelector("[data-kdoc-comment]");
    return {
      separatorTextNodes: Array.from(comment?.childNodes ?? []).filter(
        (node) => node.nodeType === Node.TEXT_NODE && /\S|\n/.test(node.textContent ?? ""),
      ).length,
      lines: Array.from(root.querySelectorAll(".kdoc-comment-line")).map((line) => ({
        text: line.textContent ?? "",
        prefix: line.querySelector(".kdoc-comment-prefix")?.textContent ?? "",
        body: line.querySelector(".kdoc-comment-body")?.textContent ?? "",
      })),
    };
  });

  const commentLines = wrappedComment.lines;
  expect(wrappedComment.separatorTextNodes).toBe(0);
  expect(commentLines.length).toBeGreaterThan(2);
  expect(commentLines.map((line) => line.prefix)).toEqual(Array(commentLines.length).fill("    # "));
  expect(commentLines.every((line) => line.text.startsWith("    # "))).toBeTruthy();
  expect(commentLines.map((line) => line.body.trim()).join(" ")).toContain(
    "experimental groups opt-in preview features",
  );
});

test("expands collapsed metadata while preloading the full payload", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");
  await mountedHost(page);

  const metadataName = page.locator('[data-kdoc-field][data-path="metadata.name"]');
  await expect(metadataName).toBeHidden();
  const metadata = page.locator('[data-path="metadata"]').first();
  await metadata.locator("[data-kdoc-toggle]").click();

  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(metadataName).toBeVisible();
  await expect(page.locator('[data-kdoc-field][data-path="metadata.annotations"]')).toBeVisible();
  await expect.poll(() => fullPayloadRequests).toBe(1);
});

test("activates the full sidecar without materializing collapsed descendants", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  const initialLineCount = await host.locator("[data-kdoc-line]").count();

  const activation = await waitForPerfEntry(page, "full-schema-activate");
  expect(activation.duration).toBeLessThanOrEqual(performanceBudgetMs);
  expect(perfNumber(activation, "lines")).toBeGreaterThan(5_000);
  expect(perfNumber(activation, "renderedLines")).toBe(initialLineCount);
  await expect.poll(() => fullPayloadRequests).toBe(1);
  await expect(host.locator("[data-kdoc-line]")).toHaveCount(initialLineCount);
});

test("mounts and lazy-loads only the active version", async ({ page }) => {
  const fullPayloads: string[] = [];
  await page.route("**/*-full.json", async (route) => {
    fullPayloads.push(route.request().url());
    await route.continue();
  });
  await page.goto("/");

  let host = await mountedHost(page);
  await expect(page.locator(".kdoc-react-host")).toHaveCount(1);
  await expect.poll(() => fullPayloads.length).toBe(1);
  expect(fullPayloads[0]).toContain("schema-0-full.json");

  await page.getByRole("tab", { name: "nvidia.com/v1alpha1" }).click();
  host = await mountedHost(page);
  await expect(page.locator(".kdoc-react-host")).toHaveCount(1);
  await expect.poll(() => fullPayloads.length).toBe(2);
  expect(fullPayloads[1]).toContain("schema-1-full.json");
});

test("preloads the full sidecar after initial mount and reuses it", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  await expect.poll(() => fullPayloadRequests).toBe(1);
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec"]').first(),
  ).toHaveCount(0);

  await host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("secretKeyRef");
  });
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]').first(),
  ).toBeVisible({ timeout: 10_000 });
  expect(fullPayloadRequests).toBe(1);
});

test("projects only visible full-schema lines when expanding a lazy branch", async ({ page }) => {
  await page.goto("/");
  const host = await mountedHost(page);

  const podTemplate = page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate"]').first();
  await expect(podTemplate.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await podTemplate.locator("[data-kdoc-toggle]").click();

  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec"]').first(),
  ).toBeVisible({ timeout: 10_000 });
  const projection = await waitForPerfEntry(page, "projection-render");
  const lines = perfNumber(projection, "lines");
  expect(projection.duration).toBeLessThanOrEqual(performanceBudgetMs);
  expect(lines).toBeLessThan(1_500);
  expect(perfNumber(projection, "fields")).toBeLessThan(500);
  expect(await host.locator("[data-kdoc-line]").count()).toBe(lines);
});

test("fold buttons handle embedded click propagation", async ({ page }) => {
  await page.goto("/");
  await mountedHost(page);

  await page.locator(".kdoc-tree").first().evaluate((tree) => {
    tree.addEventListener("click", (event) => event.stopPropagation());
  });

  const metadata = page.locator('[data-path="metadata"]').first();
  const toggle = metadata.locator("[data-kdoc-toggle]");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
});

test("releases a stale Fern cookie backdrop that covers fold buttons", async ({ page }) => {
  await page.addInitScript(() => {
    window.addEventListener("DOMContentLoaded", () => {
      const backdrop = document.createElement("div");
      backdrop.className = "onetrust-pc-dark-filter";
      Object.assign(backdrop.style, {
        position: "fixed",
        inset: "0",
        zIndex: "2147483645",
        pointerEvents: "auto",
      });
      document.body.appendChild(backdrop);
    });
  });
  await page.goto("/");
  await mountedHost(page);

  await expect.poll(() =>
    page.locator(".onetrust-pc-dark-filter").evaluate((backdrop) => getComputedStyle(backdrop).pointerEvents),
  ).toBe("none");

  const metadata = page.locator('[data-path="metadata"]').first();
  const toggle = metadata.locator("[data-kdoc-toggle]");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
});

test("clears stale selected fields before focusing another field", async ({ page }) => {
  await page.goto("/");
  await mountedHost(page);

  const apiVersion = page.locator('[data-kdoc-field][data-path="apiVersion"]').first();
  const metadata = page.locator('[data-kdoc-field][data-path="metadata"]').first();
  await apiVersion.evaluate((line) => line.classList.add("kdoc-selected"));
  await metadata.click();

  await expect(apiVersion).not.toHaveClass(/kdoc-selected/);
  await expect(metadata).toHaveClass(/kdoc-selected/);
  await expect(page.locator(".kdoc-react-host").first().locator(".kdoc-selected")).toHaveCount(1);
});

test("loads the full sidecar when filtering for collapsed descendants", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("secretKeyRef");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("secretKeyRef");

  await expect(page.locator(".kdoc-filter-overlay")).toContainText("secretKeyRef");
  const secretKeyRef = page
    .locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]')
    .first();
  await expect(secretKeyRef).toBeVisible({ timeout: 10_000 });
  await expect(secretKeyRef.locator(".kdoc-yaml-key")).toContainText("secretKeyRef");
  expect(await host.locator("[data-kdoc-line]").count()).toBeLessThan(1_500);
  expect(fullPayloadRequests).toBe(1);

  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("podTemplate");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("podTemplate");
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers"]').first(),
  ).toBeVisible({ timeout: 10_000 });
  expect(fullPayloadRequests).toBe(1);
});

test("keeps visible rows when typing an incremental filter", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  await host.click();
  await page.keyboard.press("s");

  await expect(page.locator(".kdoc-filter-overlay")).toContainText("s");
  await expect.poll(() => visibleSchemaLineCount(host)).toBeGreaterThan(0);
  await expect(page.locator('[data-kdoc-field][data-path="spec"]').first()).toBeVisible();
});

test("keeps the current view visible while a typed filter waits for the full sidecar", async ({ page }) => {
  let releaseSidecar: (() => void) | undefined;
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await new Promise<void>((resolve) => {
      releaseSidecar = resolve;
    });
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  await host.click();
  await page.keyboard.type("secretKeyRef");

  await expect(page.locator(".kdoc-filter-overlay")).toContainText("secretKeyRef");
  await expect.poll(() => fullPayloadRequests).toBe(1);
  const visibleWhileLoading = await visibleSchemaLineCount(host);
  expect(visibleWhileLoading).toBeGreaterThan(0);

  releaseSidecar?.();
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]'),
  ).toBeVisible({ timeout: 10_000 });
});

test("preserves expanded state across React full-sidecar remounts", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/?statefulFullLoad=1");
  await mountedHost(page);

  const podTemplate = page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate"]').first();
  const podSpec = page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec"]').first();
  await expect(podTemplate.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await expect(podSpec).toHaveCount(0);

  await podTemplate.locator("[data-kdoc-toggle]").click();

  await expect(podTemplate.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(podSpec).toBeVisible({ timeout: 10_000 });
  await expect.poll(() => fullPayloadRequests).toBe(1);
  await page.waitForTimeout(1_000);
  await expect(podTemplate.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(podSpec).toBeVisible();
  expect(fullPayloadRequests).toBe(1);
});

test("keeps focused details visible across React full-sidecar remounts", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await new Promise((resolve) => setTimeout(resolve, 250));
    await route.continue();
  });
  await page.goto("/?statefulFullLoad=1");
  await mountedHost(page);

  const apiVersion = page.locator('[data-kdoc-field][data-path="apiVersion"]').first();
  await apiVersion.click();

  const details = page.locator(".kdoc-details");
  await expect(details).toBeVisible();
  await expect(details).toContainText("apiVersion");
  await expect.poll(() => fullPayloadRequests).toBe(1);
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec"]').first(),
  ).toHaveCount(1, { timeout: 10_000 });
  await expect(details).toBeVisible();
  await expect(details).toContainText("apiVersion");

  await page.locator('[data-kdoc-field][data-path="kind"]').first().click();
  await expect(details).toBeVisible();
  await expect(details).toContainText("kind");
});

test("keeps comment wrapping sane after stateful filtering loads the full sidecar", async ({ page }) => {
  await page.goto("/?statefulFullLoad=1");

  const host = await mountedHost(page);
  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("secretKeyRef");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("secretKeyRef");
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec.containers[].env[].valueFrom.secretKeyRef"]'),
  ).toBeVisible({ timeout: 10_000 });

  await host.focus();
  await page.keyboard.press("Escape");
  await expect(page.locator(".kdoc-filter-overlay")).toBeHidden();
  const rootCommentBodies = page.locator('[data-detail-id="root-description"] .kdoc-comment-body');
  const rootBodies = await rootCommentBodies.allTextContents();
  expect(rootBodies.some((text) => text.includes("DynamoGraphDeployment"))).toBeTruthy();
  expect(rootBodies.some((text) => text.trim().length > 40)).toBeTruthy();
  expect(rootBodies.length).toBeLessThan(30);
});

test("preserves indentation for commented fields while filtering", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("annotations");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("annotations");

  const annotations = page.locator('[data-kdoc-field][data-path="metadata.annotations"]').first();
  const yamlText = annotations.locator(".kdoc-yaml-text");
  await expect(annotations).toBeVisible();
  await expect(annotations).toHaveCSS("display", "grid");
  await expect(yamlText).toHaveCSS("white-space", "pre-wrap");
  await expect(yamlText).not.toHaveClass(/kdoc-yaml-comment-text/);
  await expect(yamlText).toContainText("  # annotations:");
});

test("keeps fold controls interactive while filtering", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("annotations");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("annotations");
  await expect(
    page.locator('[data-kdoc-field][data-path="spec.components[].podTemplate.spec"]').first(),
  ).toHaveCount(1, { timeout: 10_000 });
  await expect.poll(() => fullPayloadRequests).toBe(1);

  const metadata = page.locator('[data-kdoc-field][data-path="metadata"]').first();
  const annotations = page.locator('[data-kdoc-field][data-path="metadata.annotations"]').first();
  const annotationValue = page.locator('[data-kdoc-field][data-path="metadata.annotations.<key>"]').first();

  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(annotations).toBeVisible();
  await metadata.locator("[data-kdoc-toggle]").click();
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await expect(annotations).toBeHidden();

  await metadata.locator("[data-kdoc-toggle]").click();
  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(annotations).toBeVisible();
  await expect(annotations.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(annotationValue).toBeVisible();

  await annotations.click();
  await page.keyboard.press("Enter");
  await expect(host.locator("[data-kdoc-filter-overlay]")).toBeHidden();
  await expect.poll(() => selectedFieldPath(host)).toBe("metadata.annotations");
  await expect(annotations.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(annotationValue).toBeVisible();
});

test("shows Fern-style focused field details overlay", async ({ page }) => {
  await page.goto("/");

  await page.locator(".kdoc-line").filter({ hasText: "apiVersion" }).first().click();

  const details = page.locator(".kdoc-details");
  await expect(details).toBeVisible();
  await expect(details).toContainText("Path");
  await expect(details).toContainText("apiVersion");
  await expect(details).toContainText("Required");
});
