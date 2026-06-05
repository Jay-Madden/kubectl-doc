import { expect, test } from "@playwright/test";

test("renders YAML token markup as DOM nodes", async ({ page }) => {
  await page.goto("/");

  const tree = page.locator(".kdoc-tree").first();
  await expect(tree).toContainText("apiVersion: nvidia.com/v1beta1");
  await expect(tree).not.toContainText("kdoc-yaml-key");
  await expect(tree.locator(".kdoc-yaml-key").filter({ hasText: "apiVersion" })).toHaveCount(1);
  await expect(tree.locator(".kdoc-yaml-punct").first()).toBeVisible();
});

test("loads the full sidecar when filtering for collapsed descendants", async ({ page }) => {
  await page.goto("/");

  const host = page.locator(".kdoc-fern-host").first();
  await host.evaluate((node) => {
    const controller = (node as HTMLElement & { __kubectlDocController?: { setFilter: (value: string) => void } })
      .__kubectlDocController;
    controller?.setFilter("podTemplate");
  });

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
