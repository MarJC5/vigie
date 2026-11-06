import { expect, test } from "@playwright/test";

// The tab Assistants on an empty Vigie (ADR-0043): off at first, the level kept once chosen, the
// notice of the level act. The server build's vigie is not linked: no lines for a client, the way
// to General instead. On the servers of the background test, whose settings it does not touch.
test("the level of the assistants is kept, and without the command the tab leads to General", async ({ page }, info) => {
  const url = info.project.name === "webkit" ? "http://localhost:34128/" : "http://localhost:34127/";
  await page.goto(url);
  const rail = page.getByRole("navigation", { name: "Navigation" });
  await rail.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Assistants" }).click();

  const levels = page.getByRole("group", { name: "What an assistant may do" });
  await expect(levels.getByRole("button", { name: /^Off/ })).toHaveAttribute("aria-pressed", "true");
  await levels.getByRole("button", { name: /^Read/ }).click();
  await expect(levels.getByRole("button", { name: /^Read/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("Let your client ask you")).toHaveCount(0);
  await levels.getByRole("button", { name: /^Actions/ }).click();
  await expect(page.getByText("Let your client ask you")).toBeVisible();

  await page.reload();
  await rail.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Assistants" }).click();
  await expect(page.getByRole("group", { name: "What an assistant may do" }).getByRole("button", { name: /^Actions/ })).toHaveAttribute("aria-pressed", "true");

  await expect(page.getByText("The vigie command is not linked")).toBeVisible();
  await page.getByRole("button", { name: "Open General" }).click();
  await expect(page.getByRole("tab", { name: "General" })).toHaveAttribute("aria-selected", "true");
});
