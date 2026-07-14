import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getAccessToken,
  setAccessToken,
} from "@/lib/access-token";
import {
  ApiError,
  getWatchlist,
  verifyAccessToken,
} from "@/lib/api-client";
import { AccessControl } from "./access-control";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return { ...actual, verifyAccessToken: vi.fn() };
});

describe("AccessControl", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.mocked(verifyAccessToken).mockReset();
    window.sessionStorage.clear();
  });

  it("unlocks after the token is verified", async () => {
    const reloadPage = vi.fn();
    vi.mocked(verifyAccessToken).mockResolvedValue({ status: "unlocked" });

    render(<AccessControl reloadPage={reloadPage} />);
    fireEvent.click(screen.getByRole("button", { name: "解锁" }));
    fireEvent.change(screen.getByLabelText("访问令牌"), {
      target: { value: "  secret-token  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "验证并解锁" }));

    await waitFor(() => {
      expect(verifyAccessToken).toHaveBeenCalledWith("secret-token");
      expect(getAccessToken()).toBe("secret-token");
      expect(reloadPage).toHaveBeenCalledOnce();
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps the session locked and displays verification failures", async () => {
    const reloadPage = vi.fn();
    vi.mocked(verifyAccessToken).mockRejectedValue(
      new ApiError("access_required", "unauthorized", 401),
    );

    render(<AccessControl reloadPage={reloadPage} />);
    fireEvent.click(screen.getByRole("button", { name: "解锁" }));
    fireEvent.change(screen.getByLabelText("访问令牌"), {
      target: { value: "wrong-token" },
    });
    fireEvent.click(screen.getByRole("button", { name: "验证并解锁" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("访问令牌无效。");
    expect(getAccessToken()).toBeUndefined();
    expect(reloadPage).not.toHaveBeenCalled();
  });

  it("clears the session and returns to locked state after a 401", async () => {
    setAccessToken("expired-token");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: "access_required",
            message: "valid bearer access token is required",
          },
        }),
        {
          headers: { "content-type": "application/json" },
          status: 401,
        },
      ),
    );

    render(<AccessControl reloadPage={vi.fn()} />);
    expect(screen.getByRole("button", { name: "已解锁" })).toBeInTheDocument();

    let requestError: unknown;
    await act(async () => {
      try {
        await getWatchlist();
      } catch (caught) {
        requestError = caught;
      }
    });
    expect(requestError).toMatchObject({ status: 401 });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "解锁" })).toBeInTheDocument();
    });
    expect(getAccessToken()).toBeUndefined();
  });

  it("locks the personal session on explicit exit", async () => {
    const reloadPage = vi.fn();
    setAccessToken("secret-token");
    render(<AccessControl reloadPage={reloadPage} />);

    fireEvent.click(screen.getByRole("button", { name: "已解锁" }));
    fireEvent.click(screen.getByRole("button", { name: "锁定个人空间" }));

    expect(getAccessToken()).toBeUndefined();
    expect(reloadPage).toHaveBeenCalledOnce();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "解锁" })).toBeInTheDocument();
    });
  });

  it("restores the unlocked state after the component is remounted", async () => {
    setAccessToken("secret-token");
    const first = render(<AccessControl reloadPage={vi.fn()} />);
    expect(screen.getByRole("button", { name: "已解锁" })).toBeInTheDocument();

    first.unmount();
    render(<AccessControl reloadPage={vi.fn()} />);
    expect(screen.getByRole("button", { name: "已解锁" })).toBeInTheDocument();
    expect(getAccessToken()).toBe("secret-token");
  });

  it("opens and closes the token dialog without changing the session", async () => {
    render(<AccessControl />);
    expect(screen.getByRole("button", { name: "解锁" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "解锁" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("访问令牌")).toHaveAttribute("type", "password");

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(getAccessToken()).toBeUndefined();
  });
});
