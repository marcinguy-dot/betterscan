import { test, expect } from "@playwright/test";

const email = `e2e-next-${Date.now()}@example.com`;
const password = "password12345";

test.describe("Next.js auth + dashboard", () => {
  test("register, land on dashboard, navigate to projects", async ({ page }) => {
    await page.goto("/register");
    await page.getByTestId("register-email").fill(email);
    await page.getByTestId("register-password").fill(password);
    await page.getByTestId("register-submit").click();
    // After register, next-auth credentials sign-in should redirect home
    await expect(page).toHaveURL(/\/($|\?)/, { timeout: 30_000 });
    await expect(page.getByText(/Security Dashboard|Dashboard|Projects/i).first()).toBeVisible({
      timeout: 20_000,
    });

    await page.goto("/projects");
    await expect(page.getByTestId("add-project-btn")).toBeVisible();
  });
});
