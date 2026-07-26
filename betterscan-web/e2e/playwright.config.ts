import { defineConfig, devices } from "@playwright/test";

const nextURL = process.env.NEXT_BASE_URL || "http://localhost:3000";
const jqueryURL = process.env.JQUERY_BASE_URL || "http://localhost:8081";

export default defineConfig({
  testDir: "./tests",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "nextjs",
      use: { ...devices["Desktop Chrome"], baseURL: nextURL },
      testMatch: /.*\.next\.spec\.ts/,
    },
    {
      name: "jquery",
      use: { ...devices["Desktop Chrome"], baseURL: jqueryURL },
      testMatch: /.*\.jquery\.spec\.ts/,
    },
  ],
});
