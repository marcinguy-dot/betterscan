import { test, expect } from "@playwright/test";

const email = `e2e-proj-${Date.now()}@example.com`;
const password = "password12345";

test.describe("Next.js projects", () => {
  test("create project with public repo URL", async ({ page }) => {
    await page.goto("/register");
    await page.getByTestId("register-email").fill(email);
    await page.getByTestId("register-password").fill(password);
    await page.getByTestId("register-submit").click();
    await expect(page).toHaveURL(/\/($|\?)/, { timeout: 30_000 });

    await page.goto("/projects");
    await page.getByTestId("add-project-btn").click();
    await page.getByTestId("project-name").fill("e2e-sample");
    await page.getByTestId("project-repo-url").fill("https://github.com/aquasecurity/trivy-action.git");
    await page.getByTestId("project-branch").fill("master");
    await page.getByTestId("project-create-btn").click();
    await expect(page.getByText("e2e-sample").first()).toBeVisible({ timeout: 20_000 });
  });
});
