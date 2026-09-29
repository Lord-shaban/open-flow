import { test, expect } from "@playwright/test";
import { stat } from "node:fs/promises";

test("owner unlock, encrypted connections and durable private creation", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(
    page.getByRole("heading", {
      name: "Where will your imagination take you?",
    }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Unlock workspace", exact: true })
    .click();
  await page.getByLabel("Owner access token").fill("incorrect-token");
  await page
    .getByRole("button", { name: "Unlock workspace", exact: true })
    .last()
    .click();
  await expect(
    page.getByRole("alert").filter({ hasText: "Access token does not match" }),
  ).toBeVisible();
  await page
    .getByLabel("Owner access token")
    .fill(process.env.OPEN_FLOW_OWNER_TOKEN!);
  await page
    .getByRole("button", { name: "Unlock workspace", exact: true })
    .last()
    .click();
  await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
  const cookies = await page.context().cookies();
  const session = cookies.find((c) => c.name === "open-flow-session");
  expect(session?.httpOnly).toBe(true);
  expect(session?.sameSite).toBe("Strict");

  await page.getByRole("button", { name: "Provider settings" }).click();
  await page
    .getByLabel("Provider", { exact: true })
    .last()
    .selectOption("horde");
  const connection = `Fixture ${testInfo.project.name}`;
  await page.getByLabel("Connection name").fill(connection);
  await page
    .getByLabel("API key", { exact: true })
    .fill("fixture-key-never-call-provider");
  const saved = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/flow/credentials") &&
      r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Save encrypted connection" }).click();
  const response = await saved;
  expect(response.status()).toBe(201);
  expect(await response.text()).not.toContain(
    "fixture-key-never-call-provider",
  );
  await expect(page.getByLabel("API key", { exact: true })).toHaveValue("");
  await page.getByRole("button", { name: `Revoke ${connection}` }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Credential revoked" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close dialog" }).click();

  await page.getByLabel("Provider", { exact: true }).selectOption("training");
  await expect(page.getByLabel("Model", { exact: true })).toHaveValue(
    "training-landscape-v1",
  );
  await page
    .getByLabel("Image prompt")
    .fill(`Private training landscape ${testInfo.project.name}`);
  await page.getByLabel("Aspect ratio").selectOption("16:9");
  await page.getByLabel("Image prompt").press("Control+Enter");
  const card = page.getByRole("button", {
    name: /Training canvas creation.*training-landscape-v1/,
  });
  await expect(card).toContainText("succeeded", { timeout: 60_000 });
  await expect(card.getByAltText("Your generated image")).toBeVisible();
  await page.reload();
  await expect(card.getByAltText("Your generated image")).toBeVisible();
  await card.click();
  const download = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download image" }).click();
  const file = await download;
  const path = await file.path();
  expect((await stat(path!)).size).toBeGreaterThan(1000);
  expect(file.suggestedFilename()).toMatch(/\.png$/);
  await page.getByRole("button", { name: "Remove from library" }).click();
  await expect(card).toHaveCount(0);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth,
  );
  expect(overflow).toBe(false);
  await page.screenshot({
    path: testInfo.outputPath("studio.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(
    page.getByRole("button", { name: "Unlock workspace", exact: true }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test("fixture history exposes failed and uncertain outcomes with keyboard access", async ({
  page,
}) => {
  await page.route("**/api/session", (route) =>
    route.fulfill({ json: { authenticated: true, configured: true } }),
  );
  await page.route("**/api/flow/models?*", (route) =>
    route.fulfill({ json: { models: [] } }),
  );
  await page.route("**/api/flow/credentials", (route) =>
    route.fulfill({ json: [] }),
  );
  const common = {
    provider: "horde",
    model_id: "fixture-image",
    aspect_ratio: "1:1",
    artifacts: [],
    route_reason: "Explicit free provider fixture",
    created_at: new Date().toISOString(),
  };
  await page.route("**/api/flow/generations", (route) =>
    route.fulfill({
      json: {
        generations: [
          {
            ...common,
            id: "00000000-0000-4000-8000-000000000010",
            state: "failed",
            error_code: "quota",
          },
          {
            ...common,
            id: "00000000-0000-4000-8000-000000000011",
            state: "reconciliation_required",
            error_code: "ambiguous_submission",
          },
          {
            ...common,
            id: "00000000-0000-4000-8000-000000000012",
            state: "queued",
          },
        ],
        next_cursor: "",
      },
    }),
  );
  await page.goto("/");
  await page
    .getByRole("button", { name: "Failed", exact: true })
    .press("Enter");
  await expect(
    page.getByRole("button", { name: /AI Horde creation/ }),
  ).toHaveCount(2);
  await page
    .getByRole("button", { name: /AI Horde creation.*ambiguous_submission/ })
    .press("Enter");
  await expect(page.getByRole("dialog")).toContainText(
    "reconciliation required",
  );
  await expect(page.getByRole("dialog")).toContainText(
    "Explicit free provider fixture",
  );
  await page.getByRole("dialog").press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: /In progress/ }).press("Enter");
  await expect(
    page.getByRole("button", { name: /AI Horde creation/ }),
  ).toContainText("queued");
});
