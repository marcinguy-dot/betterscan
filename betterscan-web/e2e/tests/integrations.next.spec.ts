import { test, expect } from "@playwright/test";

const email = `e2e-int-${Date.now()}@example.com`;
const password = "password12345";

test.describe("Next.js integrations (GitHub mock)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/register");
    await page.getByTestId("register-email").fill(email);
    await page.getByTestId("register-password").fill(password);
    await page.getByTestId("register-submit").click();
    await expect(page).toHaveURL(/\/($|\?)/, { timeout: 30_000 });
  });

  test("connect mock GitHub App", async ({ page }) => {
    await page.goto("/integrations/github");
    await expect(page.getByTestId("integrations-page")).toBeVisible();
    const btn = page.getByTestId("github-install-btn");
    await expect(btn).toBeEnabled({ timeout: 15_000 });
    await btn.click();
    await expect(page.getByTestId("connections-list")).toContainText(/mock|GitHub/i, {
      timeout: 15_000,
    });
  });
});
