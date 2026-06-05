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
  await expect(tree.locator(".kdoc-yaml-key").filter({ hasText: "apiVersion" })).toHaveCount(1);
  await expect(tree.locator(".kdoc-yaml-punct").first()).toBeVisible();
});

test("keeps Fern comments wrapped without exposing a wrap toggle", async ({ page }) => {
  await page.goto("/");

  const host = await mountedHost(page);
  await expect(host).toHaveClass(/kdoc-wrap-comments/);
  await expect(page.locator(".kdoc-view-controls")).toHaveCount(0);
  await expect(page.locator(".kdoc-wrap-toggle")).toHaveCount(0);
});

test("expands collapsed metadata after loading the full sidecar", async ({ page }) => {
  await page.goto("/");
  await mountedHost(page);

  await expect(page.locator('[data-kdoc-field][data-path="metadata.name"]')).toHaveCount(0);
  const metadata = page.locator('[data-path="metadata"]').first();
  await metadata.locator("[data-kdoc-toggle]").click();

  await expect(metadata.locator("[data-kdoc-toggle]")).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator('[data-kdoc-field][data-path="metadata.name"]')).toBeVisible({ timeout: 10_000 });
  await expect(page.locator('[data-kdoc-field][data-path="metadata.annotations"]')).toBeVisible();
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

test("shows Fern-style focused field details overlay", async ({ page }) => {
  await page.goto("/");

  await page.locator(".kdoc-line").filter({ hasText: "apiVersion" }).first().click();

  const details = page.locator(".kdoc-details");
  await expect(details).toBeVisible();
  await expect(details).toContainText("Path");
  await expect(details).toContainText("apiVersion");
  await expect(details).toContainText("Required");
});
