import { expect, type Locator, type Page, test } from "@playwright/test";

const performanceBudgetMs = 200;

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

test("keeps Fern comments wrapped without exposing a wrap toggle", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
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

test("semantically wraps single long comments with comment prefixes", async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 900 });
  await page.goto("/");
  await mountedHost(page);

  const wrappedComment = await page.evaluate(() => {
    const root = document.createElement("div");
    root.className = "kubectl-doc kdoc-react-host";
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
  await host.focus();

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
  await host.focus();
  await expect.poll(() => fullPayloads.length).toBe(1);
  expect(fullPayloads[0]).toContain("schema-0-full.json");

  await page.getByRole("tab", { name: "nvidia.com/v1alpha1" }).click();
  host = await mountedHost(page);
  await expect(page.locator(".kdoc-react-host")).toHaveCount(1);
  await host.focus();
  await expect.poll(() => fullPayloads.length).toBe(2);
  expect(fullPayloads[1]).toContain("schema-1-full.json");
});

test("preloads the full sidecar on first focus and reuses it", async ({ page }) => {
  let fullPayloadRequests = 0;
  await page.route("**/*-full.json", async (route) => {
    fullPayloadRequests++;
    await route.continue();
  });
  await page.goto("/");

  const host = await mountedHost(page);
  expect(fullPayloadRequests).toBe(0);

  await host.focus();
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
  await expect(annotations.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "false");
  await expect(annotationValue).toBeHidden();
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
