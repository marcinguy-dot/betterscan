import { test, expect } from "@playwright/test";

const email = `e2e-jq-${Date.now()}@example.com`;
const password = "password12345";

test.describe("jQuery auth + projects", () => {
  test("register and open projects", async ({ page }) => {
    await page.goto("/register.html");
    await page.locator("#email").fill(email);
    await page.locator("#password").fill(password);
    await page.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(/index\.html|\/$|8081\/?$/, { timeout: 30_000 });

    await page.goto("/projects.html");
    await expect(page.getByTestId("add-project-btn")).toBeVisible({ timeout: 15_000 });
  });
});
