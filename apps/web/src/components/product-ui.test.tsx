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

  it("labels the dashboard preview as sample data (HOME-001)", () => {
    render(createElement(HomePage));

    expect(screen.getByText("示例数据")).toBeInTheDocument();
    expect(screen.getByText("示例结论")).toBeInTheDocument();
    expect(
      screen.getByText(/以上为演示示例，非你的个人测算结果/),
    ).toBeInTheDocument();
  });

  it("does not overclaim real-data coverage in prototype copy (HOME-002)", () => {
    render(createElement(HomePage));

    expect(screen.queryByText(/不看营销软文，只看真实数据/)).not.toBeInTheDocument();
    expect(screen.queryByText(/精准计算安全总价/)).not.toBeInTheDocument();
  });
});

describe("CalculatorPanel", () => {
  const fillCoreInput = (targetTotalPrice: string, monthlyIncome = "3.5") => {
    fireEvent.change(screen.getByLabelText("家庭月收入 (万)"), {
      target: { value: monthlyIncome },
    });
    fireEvent.change(screen.getByLabelText("目标总价（万）"), {
      target: { value: targetTotalPrice },
    });
  };

  it("starts with an empty form and no personal numbers until core input is provided", () => {
    render(createElement(CalculatorPanel));

    // 不再预填任何虚构家庭财务数据（CALC-001）。
    expect(screen.getByLabelText("目标总价（万）")).toHaveValue("");
    expect(screen.getByLabelText("家庭月收入 (万)")).toHaveValue("");
    expect(
      screen.getByText(/填写左侧.*生成你的换房压力诊断/),
    ).toBeInTheDocument();
    // 无固定个人结论（CALC-005）。
    expect(screen.queryByText(/占比超 60%/)).not.toBeInTheDocument();
  });

  it("updates the diagnosis when target price becomes unsafe", () => {
    render(createElement(CalculatorPanel));

    fillCoreInput("720");

    expect(screen.getByText("危险")).toBeInTheDocument();
    expect(screen.getByText("暂缓改善")).toBeInTheDocument();
  });

  it("uses one formula for 550 with no reference-scenario override", () => {
    // CALC-003：549 / 550 / 551 万不得存在特殊代码路径。
    const readMetrics = () => {
      const metrics = screen
        .getAllByText(/^\d+(?:\.\d+)?$/)
        .map((node) => node.textContent);
      return metrics.join("|");
    };

    render(createElement(CalculatorPanel));
    fillCoreInput("549");
    const at549 = readMetrics();
    fillCoreInput("550");
    const at550 = readMetrics();
    fillCoreInput("551");
    const at551 = readMetrics();

    // 550 不再触发 520/约35/42% 的固定覆盖，与相邻取值连续。
    expect(new Set([at549, at550, at551]).size).toBe(3);
    expect(screen.queryByText("约 35")).not.toBeInTheDocument();
  });

  it("exposes the previously hidden mortgage and transaction-cost fields", () => {
    // CALC-002：两个参与计算的字段必须可见可编辑。
    render(createElement(CalculatorPanel));

    expect(screen.getByLabelText("当前月供（元）")).toBeInTheDocument();
    expect(screen.getByLabelText("交易税费（万）")).toBeInTheDocument();
  });

  it("links the method cards to a real, keyboard-reachable page", () => {
    // CALC-007：方法卡片是真实链接而非假 <article>。
    render(createElement(CalculatorPanel));

    const link = screen.getByRole("link", {
      name: /为什么月供安全线比总价更重要/,
    });
    expect(link).toHaveAttribute("href", "/methods");
  });

  it("posts current input and displays the API diagnosis when regenerated", async () => {
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce({
      id: "calculation_1",
      result: {
        pressureLevel: "safe",
        strategy: "可以同步推进",
        ruleVersion: "2026.07",
        effectiveDate: "2026-07-01",
      },
    });
    render(createElement(CalculatorPanel));

    fillCoreInput("500");
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

    fillCoreInput("720");
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

  it("lets the user tick off checklist items (ACTION-005)", async () => {
    vi.mocked(getActionWindow).mockResolvedValueOnce({
      action: "出手",
      confidence: "高",
      summary: "预算安全且目标户型稀缺，可以准备出手。",
      checklist: ["确认贷款批复。", "准备谈价底线。"],
      risks: ["不要因为稀缺而突破预算。"],
    });
    render(createElement(ActionWindowPage));

    const checkbox = await screen.findByRole("checkbox", { name: "确认贷款批复。" });
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
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

  it("does not fake navigation for topics without content (METHOD-001)", () => {
    render(createElement(MethodsPage));

    // 已完成的主题是真实锚点。
    expect(
      screen.getByRole("link", { name: "挂牌变多但成交弱，说明什么？" }),
    ).toHaveAttribute("href", "#main-method");
    // 未完成主题不再是假链接，且明确标注即将上线。
    expect(
      screen.queryByRole("link", { name: /为什么不能只看挂牌均价/ }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("即将上线")).toHaveLength(5);
  });

  it("replaces fixed magnitudes with qualitative wording and shows provenance (METHOD-002)", () => {
    render(createElement(MethodsPage));

    // 固定幅度承诺已去除。
    expect(screen.queryByText(/超过 20%/)).not.toBeInTheDocument();
    expect(screen.queryByText(/砍 3%-5%/)).not.toBeInTheDocument();
    // 来源/适用范围与版本可见。
    expect(screen.getByText("方法适用范围与来源")).toBeInTheDocument();
    expect(screen.getByText(/规则版本 2026.07/)).toBeInTheDocument();
    expect(screen.getByText("适用范围")).toBeInTheDocument();
    expect(screen.getByText("来源")).toBeInTheDocument();
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

  it("copies the template structure to the clipboard (TEMPLATE-001)", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(createElement(TemplatesPage));

    const [firstCopyButton] = screen.getAllByRole("button", {
      name: "复制模板结构",
    });
    fireEvent.click(firstCopyButton);

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledTimes(1);
    });
    const copied = writeText.mock.calls[0][0] as string;
    expect(copied).toContain("# 换房预算表");
    expect(copied).toContain("- 现金：");
    expect(await screen.findByText("已复制到剪贴板")).toBeInTheDocument();

    vi.unstubAllGlobals();
  });
});
