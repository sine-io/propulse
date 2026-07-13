import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import RootLayout, { metadata } from "@/app/layout";
import {
  createCapacityCalculation,
  getActionWindow,
  getWatchlist,
} from "@/lib/api-client";
import { AppHeader } from "./app-header";
import { ActionWindowPage } from "./action-window-page";
import { CalculatorPanel } from "./calculator-panel";
import { HomePage } from "./home-page";
import { MethodsPage } from "./methods-page";
import { NeighborhoodsPage } from "./neighborhoods-page";
import { TemplatesPage } from "./templates-page";
import { WatchlistPage } from "./watchlist-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();

  return {
    ...actual,
    createCapacityCalculation: vi.fn(),
    getActionWindow: vi.fn(),
    getWatchlist: vi.fn(),
  };
});

beforeEach(() => {
  vi.mocked(createCapacityCalculation).mockReset();
  vi.mocked(getActionWindow).mockReset();
  vi.mocked(getWatchlist).mockReset();
  vi.mocked(createCapacityCalculation).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getActionWindow).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getWatchlist).mockRejectedValue(new Error("api unavailable"));
});

describe("AppHeader", () => {
  it("exposes all MVP navigation entries", () => {
    render(createElement(AppHeader));

    expect(screen.getByRole("link", { name: /房脉 propulse/ })).toHaveAttribute(
      "href",
      "/",
    );

    for (const label of [
      "换房测算",
      "目标小区",
      "出手窗口",
      "判断方法",
      "我的观察池",
    ]) {
      expect(screen.getByRole("link", { name: label })).toBeInTheDocument();
    }
  });

  it("exposes the prototype mobile quick navigation", () => {
    render(createElement(AppHeader));

    const quickNav = screen.getByRole("navigation", { name: "移动快捷导航" });

    for (const [label, href] of [
      ["测算", "/calculator"],
      ["小区", "/neighborhoods"],
      ["窗口", "/action-window"],
      ["方法", "/methods"],
      ["观察池", "/watchlist"],
    ]) {
      expect(
        within(quickNav).getByRole("link", { name: label }),
      ).toHaveAttribute("href", href);
    }
  });
});

describe("RootLayout", () => {
  it("uses the corrected propulse product name in metadata and footer", () => {
    const markup = renderToStaticMarkup(
      createElement(RootLayout, null, createElement("main", null, "短页面")),
    );

    const misspelledProductName = "prop" + "pulse";

    expect(metadata.title).toBe("房脉 propulse - 房产决策工具");
    expect(markup).toContain("© 2026 房脉 propulse Prototype");
    expect(markup).not.toContain(misspelledProductName);
  });

  it("allows the footer to sit at the viewport bottom on short pages", () => {
    const markup = renderToStaticMarkup(
      createElement(RootLayout, null, createElement("main", null, "短页面")),
    );

    expect(markup).toMatch(
      /<body class="[^"]*\bflex\b[^"]*\bmin-h-screen\b[^"]*\bflex-col\b/,
    );
    expect(markup).toMatch(/<footer class="[^"]*\bmt-auto\b/);
  });
});

describe("HomePage", () => {
  it("makes the product purpose and two primary entry points clear", () => {
    render(createElement(HomePage));

    expect(
      screen.getByRole("heading", {
        name: /想买房或换房，先算清压力，再判断时机/,
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "开始换房测算" })).toHaveAttribute(
      "href",
      "/calculator",
    );
    expect(screen.getByRole("link", { name: "添加目标小区" })).toHaveAttribute(
      "href",
      "/neighborhoods",
    );
    expect(screen.getByText("可以开始看房，但不急下定")).toBeInTheDocument();
  });
});

describe("CalculatorPanel", () => {
  it("updates the diagnosis when target price becomes unsafe", () => {
    render(createElement(CalculatorPanel));

    expect(screen.queryByText(/^月供压力：/)).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("目标总价（万）"), {
      target: { value: "720" },
    });

    expect(screen.getByText("危险")).toBeInTheDocument();
    expect(screen.getByText("暂缓改善")).toBeInTheDocument();
  });

  it("posts current input and displays the API diagnosis when regenerated", async () => {
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce({
      id: "calculation_1",
      result: {
        pressureLevel: "safe",
        strategy: "可以同步推进",
      },
    });
    render(createElement(CalculatorPanel));

    fireEvent.change(screen.getByLabelText("目标总价（万）"), {
      target: { value: "500" },
    });
    fireEvent.click(screen.getByRole("button", { name: "重新生成诊断报告" }));

    await waitFor(() => {
      expect(createCapacityCalculation).toHaveBeenCalledWith(
        expect.objectContaining({ targetTotalPrice: 500 }),
        expect.any(AbortSignal),
      );
    });
    expect(
      await screen.findByText((content) => content.includes("可以同步推进")),
    ).toBeInTheDocument();
    expect(screen.getByText("安全")).toBeInTheDocument();
  });

  it("shows an inline API error while preserving the local preview", async () => {
    vi.mocked(createCapacityCalculation).mockRejectedValueOnce(
      new Error("api unavailable"),
    );
    render(createElement(CalculatorPanel));

    fireEvent.change(screen.getByLabelText("目标总价（万）"), {
      target: { value: "720" },
    });
    fireEvent.click(screen.getByRole("button", { name: "重新生成诊断报告" }));

    expect(await screen.findByText("诊断报告暂时无法更新，请稍后重试。")).toBeInTheDocument();
    expect(screen.getByText("危险")).toBeInTheDocument();
  });
});

describe("NeighborhoodsPage", () => {
  it("matches the reference community signal summary", () => {
    render(createElement(NeighborhoodsPage));

    expect(screen.getByText("更新时间: 今天 10:30")).toBeInTheDocument();
    expect(screen.getByText("综合研判结论")).toBeInTheDocument();
    expect(screen.getByText("适合试探性砍价")).toBeInTheDocument();
    expect(screen.getByText("降价提醒")).toBeInTheDocument();
    expect(screen.getByText("带看转定率")).toBeInTheDocument();
    expect(
      screen.queryByText(
        "重点看 495-545 万成交区间附近房源，对挂牌久、降价过的房源试探底价。",
      ),
    ).not.toBeInTheDocument();
  });
});

describe("ActionWindowPage", () => {
	it("does not invent a recommendation when the API is unavailable", async () => {
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("暂时无法生成出手窗口")).toBeInTheDocument();
		expect(screen.getByText("决策服务暂时不可用。")).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
  });

  it("renders API action window recommendations", async () => {
    vi.mocked(getActionWindow).mockResolvedValueOnce({
      action: "出手",
      confidence: "高",
      summary: "预算安全且目标户型稀缺，可以准备出手。",
      checklist: ["确认贷款批复。", "准备谈价底线。"],
      risks: ["不要因为稀缺而突破预算。"],
    });
    render(createElement(ActionWindowPage));

    expect(await screen.findByText("建议出手")).toBeInTheDocument();
    expect(screen.getByText("高")).toBeInTheDocument();
    expect(screen.getByText("确认贷款批复。")).toBeInTheDocument();
    expect(screen.getByText("不要因为稀缺而突破预算。")).toBeInTheDocument();
  });
});

describe("MethodsPage", () => {
  it("matches the reference methodology article structure", () => {
    render(createElement(MethodsPage));

    expect(screen.getByText("问题场景目录")).toBeInTheDocument();
    expect(screen.getByText("✕")).toBeInTheDocument();
    expect(screen.queryByText("x")).not.toBeInTheDocument();
    expect(screen.getByText("常见误判")).toBeInTheDocument();
    expect(screen.getByText("你需要盯住的关键指标")).toBeInTheDocument();
    expect(screen.getByText("前往目标小区实践")).toBeInTheDocument();
  });
});

describe("WatchlistPage", () => {
	it("does not fall back to sample communities when the API fails", async () => {
		render(createElement(WatchlistPage));

		expect(screen.getByText("导出本周报表")).toBeInTheDocument();
		expect(await screen.findByText("观察池暂时无法读取。")).toBeInTheDocument();
		expect(screen.getAllByText("0")).toHaveLength(4);
		expect(screen.getByText("小区动态 (本周变化)")).toBeInTheDocument();
		expect(
			screen.queryByRole("heading", { name: "青枫花园 滨江核心 · 三房" }),
		).not.toBeInTheDocument();
		expect(screen.getByText("保存复盘记录")).toBeInTheDocument();
  });

  it("renders API watchlist items when available", async () => {
    vi.mocked(getWatchlist).mockResolvedValueOnce({
      items: [
        {
          id: "watchlist_1",
          neighborhoodId: "neighborhood_api",
          name: "接口花园",
          area: "南城",
          targetLayout: "两房",
          status: "重点看",
          listedHomes: 18,
          priceCutHomes: 6,
          transactionMomentum: "strong",
          advice: "API 返回的重点建议。",
        },
      ],
    });
    render(createElement(WatchlistPage));

    expect(
      await screen.findByRole("heading", { name: "接口花园 南城 · 两房" }),
    ).toBeInTheDocument();
    expect(screen.getByText("18套")).toBeInTheDocument();
    expect(
      screen.getByText((content) => content.includes("API 返回的重点建议。")),
    ).toBeInTheDocument();
  });

  it("renders an empty watchlist state for a successful empty API response", async () => {
    vi.mocked(getWatchlist).mockResolvedValueOnce({ items: [] });
    render(createElement(WatchlistPage));

    await waitFor(() => {
      expect(
        screen.queryByRole("heading", { name: "青枫花园 滨江核心 · 三房" }),
      ).not.toBeInTheDocument();
    });

    expect(screen.getAllByText("0")).toHaveLength(4);
    expect(screen.getByText("观察池暂无小区")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "云澜府 城东新区 · 四房" }),
    ).not.toBeInTheDocument();
  });
});

describe("TemplatesPage", () => {
  it("lists every MVP off-site decision template", () => {
    render(createElement(TemplatesPage));

    for (const title of [
      "换房预算表",
      "目标小区观察表",
      "周监测表",
      "看房记录表",
      "谈价清单",
      "决策复盘表",
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
  });
});
