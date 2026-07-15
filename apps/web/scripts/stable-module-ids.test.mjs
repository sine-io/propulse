import { describe, expect, it } from "vitest";

import { stableModuleIdentifier } from "./stable-module-ids.mjs";

describe("stableModuleIdentifier", () => {
  it("normalizes raw and URL-encoded project roots", () => {
    const localRoot = "/home/ubuntu/propulse/apps/web";
    const runnerRoot = "/home/runner/work/propulse/propulse/apps/web";
    const request = (root) => [
      `${root}/node_modules/next/loader.js`,
      `?modules=${encodeURIComponent(JSON.stringify({ request: `${root}/src/components/page.tsx` }))}`,
    ].join("");

    expect(stableModuleIdentifier(request(localRoot), localRoot)).toBe(
      stableModuleIdentifier(request(runnerRoot), runnerRoot),
    );
  });

  it("does not alter identifiers outside the project root", () => {
    expect(stableModuleIdentifier("external react", "/workspace/apps/web")).toBe("external react");
  });
});
