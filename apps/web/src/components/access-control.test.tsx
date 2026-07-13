import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAccessToken, setAccessToken } from "@/lib/access-token";
import { AccessControl } from "./access-control";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return { ...actual, verifyAccessToken: vi.fn() };
});

describe("AccessControl", () => {
  afterEach(() => {
    clearAccessToken();
  });

  it("reflects the session access state and opens the unlock dialog", async () => {
    render(<AccessControl />);
    expect(screen.getByRole("button", { name: "解锁" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "解锁" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("访问令牌")).toHaveAttribute("type", "password");

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    setAccessToken("secret-token");
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "已解锁" })).toBeInTheDocument();
    });
  });
});
