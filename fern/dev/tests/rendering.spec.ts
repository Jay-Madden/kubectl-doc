import { expect, type Locator, type Page, test } from "@playwright/test";

async function mountedHost(page: Page): Promise<Locator> {
  const host = page.locator(".kdoc-fern-host").first();
  await page.waitForFunction(() => {
    const node = document.querySelector(".kdoc-fern-host") as HTMLElement & {
      __kubectlDocController?: unknown;
    };
    return Boolean(node?.__kubectlDocController);
  });
  return host;
}

test("renders YAML token markup as DOM nodes", async ({ page }) => {
  await page.goto("/");

  const tree = page.locator(".kdoc-tree").first();
  await expect(tree).toContainText("apiVersion: nvidia.com/v1beta1");
  await expect(tree).not.toContainText("kdoc-yaml-key");
  await expect(tree.locator('[data-kdoc-field][data-path="apiVersion"] .kdoc-yaml-key')).toHaveCount(1);
  await expect(tree.locator(".kdoc-yaml-punct").first()).toBeVisible();
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

test("expands collapsed metadata from the initial payload", async ({ page }) => {
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
  expect(fullPayloadRequests).toBe(0);
});

test("loads the full sidecar when filtering for collapsed descendants", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  await expect.poll(() => host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("podTemplate");
    return node.querySelector("[data-kdoc-filter-overlay]")?.textContent ?? "";
  })).toContain("podTemplate");

  await expect(page.locator(".kdoc-filter-overlay")).toContainText("podTemplate");
  const podTemplate = page.locator(".kdoc-line").filter({ hasText: "podTemplate" }).first();
  await expect(podTemplate).toBeVisible({ timeout: 10_000 });
  await expect(podTemplate.locator(".kdoc-yaml-key")).toContainText("podTemplate");
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

test("shows Fern-style focused field details overlay", async ({ page }) => {
  await page.goto("/");

  await page.locator(".kdoc-line").filter({ hasText: "apiVersion" }).first().click();

  const details = page.locator(".kdoc-details");
  await expect(details).toBeVisible();
  await expect(details).toContainText("Path");
  await expect(details).toContainText("apiVersion");
  await expect(details).toContainText("Required");
});
