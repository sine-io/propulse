import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  createReviewNote,
  listReviewNotes,
  updateReviewNote,
  type ReviewNote,
  type WatchlistItem,
} from "@/lib/api-client";
import { getShanghaiWeekStart } from "@/lib/shanghai-date";
import { ReviewNotesSection } from "./review-notes-section";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    createReviewNote: vi.fn(),
    listReviewNotes: vi.fn(),
    updateReviewNote: vi.fn(),
  };
});

const watchlistItem = {
  neighborhoodId: "11111111-1111-4111-8111-111111111111",
  name: "接口花园",
  area: "南城",
} as WatchlistItem;

beforeEach(() => {
  vi.mocked(createReviewNote).mockReset();
  vi.mocked(listReviewNotes).mockReset();
  vi.mocked(updateReviewNote).mockReset();
});

describe("ReviewNotesSection", () => {
  it("derives the current Monday in Asia/Shanghai", () => {
    expect(getShanghaiWeekStart(new Date("2026-07-12T16:30:00Z"))).toBe("2026-07-13");
    expect(getShanghaiWeekStart(new Date("2026-07-19T15:59:59Z"))).toBe("2026-07-13");
  });

  it("shows checking and locked states without requesting private data", () => {
    const { rerender } = render(
      <ReviewNotesSection accessState="checking" watchlistItems={[]} />,
    );
    expect(screen.getByText("正在确认复盘访问状态")).toBeInTheDocument();

    rerender(<ReviewNotesSection accessState="locked" watchlistItems={[]} />);
    expect(screen.getByText("复盘记录已锁定")).toBeInTheDocument();
    expect(listReviewNotes).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("复盘正文")).not.toBeInTheDocument();
  });

  it("distinguishes loading, empty, and failed history states and retries", async () => {
    const pending = deferred<Awaited<ReturnType<typeof listReviewNotes>>>();
    vi.mocked(listReviewNotes)
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce(reviewPage([]));
    const { rerender } = render(
      <ReviewNotesSection accessState="unlocked" watchlistItems={[]} />,
    );
    expect(await screen.findByText("正在加载复盘历史")).toBeInTheDocument();

    await act(async () => pending.reject(new Error("offline")));
    expect(await screen.findByText("复盘历史读取失败")).toBeInTheDocument();
    expect(screen.getByLabelText("复盘正文")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试历史" }));
    expect(await screen.findByText("暂无复盘历史")).toBeInTheDocument();
    expect(listReviewNotes).toHaveBeenLastCalledWith(1, 10, expect.any(AbortSignal));

    rerender(<ReviewNotesSection accessState="locked" watchlistItems={[]} />);
  });

  it("creates once while saving, clears the form, and re-reads page one", async () => {
    const saved = reviewNote();
    const pending = deferred<ReviewNote>();
    vi.mocked(listReviewNotes)
      .mockResolvedValueOnce(reviewPage([]))
      .mockResolvedValueOnce(reviewPage([saved], 1));
    vi.mocked(createReviewNote).mockReturnValueOnce(pending.promise);
    render(
      <ReviewNotesSection accessState="unlocked" watchlistItems={[watchlistItem]} />,
    );

    await screen.findByText("暂无复盘历史");
    fireEvent.change(screen.getByLabelText("关联观察池小区（可选）"), {
      target: { value: watchlistItem.neighborhoodId },
    });
    fireEvent.change(screen.getByLabelText("复盘正文"), {
      target: { value: "  本周感受🙂\n下周计划  " },
    });
    const saveButton = screen.getByRole("button", { name: "保存复盘" });
    fireEvent.click(saveButton);
    fireEvent.click(saveButton);

    expect(createReviewNote).toHaveBeenCalledTimes(1);
    expect(createReviewNote).toHaveBeenCalledWith(
      {
        content: "本周感受🙂\n下周计划",
        kind: "review",
        neighborhoodId: watchlistItem.neighborhoodId,
        weekStartDate: getShanghaiWeekStart(),
      },
      expect.any(AbortSignal),
    );
    expect(screen.getByRole("button", { name: "正在保存" })).toBeDisabled();
    expect(screen.getByLabelText("复盘正文")).toHaveValue("  本周感受🙂\n下周计划  ");

    await act(async () => pending.resolve(saved));
    await waitFor(() => expect(screen.getByLabelText("复盘正文")).toHaveValue(""));
    expect(await screen.findByText("本周复盘正文")).toBeInTheDocument();
    expect(listReviewNotes).toHaveBeenLastCalledWith(1, 10, expect.any(AbortSignal));
    expect(screen.getByText("复盘已保存，历史记录已重新读取。")).toBeInTheDocument();
  });

  it("validates Unicode content and keeps all creation input after failure", async () => {
    vi.mocked(listReviewNotes).mockResolvedValueOnce(reviewPage([]));
    vi.mocked(createReviewNote).mockRejectedValueOnce(new Error("offline"));
    render(
      <ReviewNotesSection accessState="unlocked" watchlistItems={[watchlistItem]} />,
    );
    await screen.findByText("暂无复盘历史");

    fireEvent.change(screen.getByLabelText("复盘正文"), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: "保存复盘" }));
    expect(screen.getByRole("alert")).toHaveTextContent("1–8000");
    expect(createReviewNote).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("关联观察池小区（可选）"), {
      target: { value: watchlistItem.neighborhoodId },
    });
    fireEvent.change(screen.getByLabelText("复盘正文"), { target: { value: "保留这段输入" } });
    fireEvent.click(screen.getByRole("button", { name: "保存复盘" }));
    expect(await screen.findByText("复盘保存失败，输入已保留。")).toBeInTheDocument();
    expect(screen.getByLabelText("复盘正文")).toHaveValue("保留这段输入");
    expect(screen.getByLabelText("关联观察池小区（可选）")).toHaveValue(watchlistItem.neighborhoodId);
  });

  it("paginates ten at a time", async () => {
    vi.mocked(listReviewNotes)
      .mockResolvedValueOnce(reviewPage([reviewNote()], 11, 1))
      .mockResolvedValueOnce(reviewPage([reviewNote({ id: "note-11", content: "第二页正文" })], 11, 2));
    render(<ReviewNotesSection accessState="unlocked" watchlistItems={[]} />);

    await screen.findByText("本周复盘正文");
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    expect(await screen.findByText("第二页正文")).toBeInTheDocument();
    expect(listReviewNotes).toHaveBeenLastCalledWith(2, 10, expect.any(AbortSignal));
    expect(screen.getByText("第 2 / 2 页")).toBeInTheDocument();
  });

  it("reuses the editor and patches content without mutable metadata", async () => {
    const note = reviewNote();
    const updated = reviewNote({ content: "更新后的正文" });
    vi.mocked(listReviewNotes)
      .mockResolvedValueOnce(reviewPage([note], 1))
      .mockResolvedValueOnce(reviewPage([updated], 1));
    vi.mocked(updateReviewNote).mockResolvedValueOnce(updated);
    render(
      <ReviewNotesSection accessState="unlocked" watchlistItems={[watchlistItem]} />,
    );

    await screen.findByText("本周复盘正文");
    fireEvent.click(screen.getByRole("button", { name: "重新编辑" }));
    expect(screen.getAllByRole("textbox")).toHaveLength(1);
    expect(screen.getByLabelText("复盘正文")).toHaveValue("本周复盘正文");
    expect(screen.queryByLabelText("复盘周次")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("关联观察池小区（可选）")).not.toBeInTheDocument();
    expect(screen.getByText("决策复盘")).toBeInTheDocument();
    expect(screen.getByText("2026-07-13")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("复盘正文"), { target: { value: " 更新后的正文 " } });
    fireEvent.click(screen.getByRole("button", { name: "更新复盘" }));
    await waitFor(() => expect(updateReviewNote).toHaveBeenCalledWith(
      note.id,
      { content: "更新后的正文" },
      expect.any(AbortSignal),
    ));
    expect(createReviewNote).not.toHaveBeenCalled();
    expect(await screen.findByText("复盘已更新，历史记录已重新读取。")).toBeInTheDocument();
  });
});

function reviewNote(overrides: Partial<ReviewNote> = {}): ReviewNote {
  return {
    id: "33333333-3333-4333-8333-333333333333",
    kind: "review",
    neighborhoodId: watchlistItem.neighborhoodId,
    weekStartDate: "2026-07-13",
    content: "本周复盘正文",
    createdAt: "2026-07-15T01:00:00Z",
    updatedAt: "2026-07-15T02:00:00Z",
    ...overrides,
  };
}

function reviewPage(items: ReviewNote[], total = items.length, page = 1) {
  return { items, total, page, pageSize: 10 };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}
