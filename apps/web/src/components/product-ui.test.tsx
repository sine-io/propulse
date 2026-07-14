import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import RootLayout, { metadata } from "@/app/layout";
import { setAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createCapacityCalculation,
  getActionWindow,
  getCapacityAssumptions,
  getWatchlist,
  type CalculationResponse,
  type CapacityAssumptionsResponse,
  type WatchlistItem,
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
    getCapacityAssumptions: vi.fn(),
    getWatchlist: vi.fn(),
  };
});

beforeEach(() => {
	window.sessionStorage.clear();
  vi.mocked(createCapacityCalculation).mockReset();
  vi.mocked(getActionWindow).mockReset();
  vi.mocked(getCapacityAssumptions).mockReset();
  vi.mocked(getWatchlist).mockReset();
  vi.mocked(createCapacityCalculation).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getActionWindow).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getCapacityAssumptions).mockRejectedValue(new Error("api unavailable"));
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
      "数据管理",
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
      ["数据", "/data"],
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
    expect(markup).toContain("© 2026 房脉 propulse");
    expect(markup).toContain("来源、采集时间和质量状态");
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
  const assumptionsFixture = {
    ruleVersion: "2026.07.14",
    effectiveDate: "2026-07-14",
    ruleSource: "房脉测算规则",
    downPaymentRate: 0.35,
    loan: {
      annualInterestRate: 0.039,
      loanTermMonths: 360,
      repaymentMethod: "equal_installment",
    },
    loanSource: "贷款配置来源",
    loanOrigin: "configured_default",
    cityPolicy: {
      city: "测试市",
      policyName: "测试首付政策",
      downPaymentRate: 0.35,
      effectiveDate: "2026-07-14",
      source: "测试政策来源",
      origin: "configured_default",
    },
    reserveMonths: 6,
    pressureThresholds: {
      safeRatio: 0.35,
      strainedRatio: 0.45,
      dangerRatio: 0.55,
      dangerMultiplier: 1.15,
    },
    oldHomeShareThreshold: 0.5,
  } satisfies CapacityAssumptionsResponse;

  const reportFixture = {
    id: "calculation_1",
    input: {
      cashOnHand: 150,
      oldHomeValue: 320,
      oldLoanBalance: 80,
      monthlyIncome: 3.5,
      currentMonthlyMortgage: 0,
      acceptableMonthlyMortgage: 1.5,
      targetTotalPrice: 500,
      renovationBudget: 40,
      transactionCosts: 18,
      transitionRentCost: 5,
      loanOverride: assumptionsFixture.loan,
      cityPolicyOverride: {
        city: "测试市",
        policyName: "测试首付政策",
        downPaymentRate: 0.35,
        effectiveDate: "2026-07-14",
        source: "测试政策来源",
      },
    },
    result: {
      netOldHomeProceeds: 240,
      deployableCash: 306,
      safeTotalPrice: 520,
      strainedTotalPrice: 610,
      dangerTotalPrice: 700,
      downPaymentGap: 0,
      monthlyPayment: 1.27,
      monthlyPaymentRatio: 0.42,
      pressureLevel: "strained",
      minimumSafeOldHomeSalePrice: 290,
      strategy: "先卖后买或同步推进",
      reasons: ["后端返回的完整诊断原因。"],
      ruleVersion: "2026.07.14",
      effectiveDate: "2026-07-14",
      traceabilityStatus: "complete",
      appliedAssumptions: assumptionsFixture,
    },
    createdAt: "2026-07-14T12:00:00Z",
  } satisfies CalculationResponse;

  const fillAllFamilyFields = (targetTotalPrice = "500") => {
    for (const [label, value] of [
      ["当前可用现金 (万)", "150"],
      ["预期售出底价 (万)", "320"],
      ["剩余贷款 (万)", "80"],
      ["家庭月收入 (万)", "3.5"],
      ["当前月供（元）", "0"],
      ["可接受极限月供 (元)", "15000"],
      ["目标总价（万）", targetTotalPrice],
      ["装修及杂费预算 (万)", "40"],
      ["交易税费（万）", "18"],
      ["过渡租房成本 (万)", "5"],
    ]) {
      fireEvent.change(screen.getByLabelText(label, { exact: false }), { target: { value } });
    }
  };

  beforeEach(() => {
    vi.mocked(getCapacityAssumptions).mockResolvedValue(assumptionsFixture);
  });

  it("starts empty and never renders a local preview", async () => {
    render(createElement(CalculatorPanel));

    expect(screen.getByLabelText("目标总价（万）")).toHaveValue("");
    expect(screen.getByLabelText("家庭月收入 (万)")).toHaveValue("");
    expect(screen.getByText(/填写全部家庭、贷款与城市政策参数并提交/)).toBeInTheDocument();
    await screen.findByLabelText("年利率（%）");
    fireEvent.change(screen.getByLabelText("目标总价（万）"), { target: { value: "720" } });
    expect(screen.queryByText("暂缓改善")).not.toBeInTheDocument();
    expect(screen.queryByTestId("pressure-pointer")).not.toBeInTheDocument();
  });

  it("blocks submission when assumptions fail and retries successfully", async () => {
    vi.mocked(getCapacityAssumptions)
      .mockReset()
      .mockRejectedValueOnce(new Error("unavailable"))
      .mockResolvedValueOnce(assumptionsFixture);
    render(createElement(CalculatorPanel));

    expect(await screen.findByText("当前测算假设加载失败。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "生成诊断报告" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "重试加载" }));
    expect(await screen.findByLabelText("年利率（%）")).toHaveValue("3.9");
    expect(screen.getByRole("button", { name: "生成诊断报告" })).toBeEnabled();
  });

  it("requires all ten family fields while accepting explicit zero", async () => {
    render(createElement(CalculatorPanel));
    await screen.findByLabelText("年利率（%）");
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    expect(screen.getAllByText("请填写此项")).toHaveLength(10);
    expect(createCapacityCalculation).not.toHaveBeenCalled();

    fillAllFamilyFields();
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce(reportFixture);
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));
    await waitFor(() => expect(createCapacityCalculation).toHaveBeenCalled());
    expect(vi.mocked(createCapacityCalculation).mock.calls[0]?.[0].currentMonthlyMortgage).toBe(0);
  });

  it("exposes the previously hidden mortgage and transaction-cost fields", async () => {
    // CALC-002：两个参与计算的字段必须可见可编辑。
    render(createElement(CalculatorPanel));
	await screen.findByLabelText("年利率（%）");

    expect(screen.getByLabelText("当前月供（元）")).toBeInTheDocument();
    expect(screen.getByLabelText("交易税费（万）")).toBeInTheDocument();
  });

  it("links the method cards to a real, keyboard-reachable page", async () => {
    // CALC-007：方法卡片是真实链接而非假 <article>。
    render(createElement(CalculatorPanel));
	await screen.findByLabelText("年利率（%）");

    const link = screen.getByRole("link", {
      name: /为什么月供安全线比总价更重要/,
    });
    expect(link).toHaveAttribute("href", "/methods");
  });

  it("submits current loan and city policy parameters on every calculation", async () => {
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce(reportFixture);
    render(createElement(CalculatorPanel));

    const rate = await screen.findByLabelText("年利率（%）");
    expect(rate).toHaveValue("3.9");
    expect(screen.getByDisplayValue("测试市")).toBeInTheDocument();
    fireEvent.change(rate, { target: { value: "4.9" } });
    fireEvent.change(screen.getByLabelText("首付比例（%）"), { target: { value: "40" } });
    fillAllFamilyFields();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    await waitFor(() => {
      expect(createCapacityCalculation).toHaveBeenCalledWith(
        expect.objectContaining({
          loanOverride: expect.objectContaining({
            annualInterestRate: expect.closeTo(0.049, 5),
            loanTermMonths: 360,
            repaymentMethod: "equal_installment",
          }),
          cityPolicyOverride: expect.objectContaining({
            city: "测试市",
            policyName: "测试首付政策",
            downPaymentRate: 0.4,
            effectiveDate: "2026-07-14",
            source: "测试政策来源",
          }),
        }),
        expect.any(AbortSignal),
      );
    });
  });

  it("renders only the complete saved API report and positions pressure continuously", async () => {
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce(reportFixture);
    render(createElement(CalculatorPanel));
    await screen.findByLabelText("年利率（%）");
    fillAllFamilyFields();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    expect(await screen.findByText("后端返回的完整诊断原因。")).toBeInTheDocument();
    expect(screen.getByText(/报告 ID：calculation_1/)).toBeInTheDocument();
    expect(screen.getByText(/生成时间：2026-07-14T12:00:00Z/)).toBeInTheDocument();
    expect(screen.getByText("最终输入")).toBeInTheDocument();
    expect(screen.getByText("应用假设")).toBeInTheDocument();
    expect(screen.getByText("房脉测算规则")).toBeInTheDocument();
    expect(screen.getByText(/测试政策来源/)).toBeInTheDocument();

    const pointer = screen.getByTestId("pressure-pointer");
    const expectedLeft = (0.42 / (0.55 * 1.1)) * 100;
    expect(pointer).toHaveStyle({ left: `${expectedLeft}%` });
  });

  it("clears a saved report as soon as any input changes", async () => {
    vi.mocked(createCapacityCalculation).mockResolvedValueOnce(reportFixture);
    render(createElement(CalculatorPanel));
    await screen.findByLabelText("年利率（%）");
    fillAllFamilyFields();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));
    expect(await screen.findByText("后端返回的完整诊断原因。")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("交易税费（万）"), { target: { value: "19" } });
    expect(screen.queryByText("后端返回的完整诊断原因。")).not.toBeInTheDocument();
    expect(screen.getByText(/填写全部家庭、贷款与城市政策参数并提交/)).toBeInTheDocument();
  });

  it("shows an API error without retaining or inventing a conclusion", async () => {
    vi.mocked(createCapacityCalculation).mockRejectedValueOnce(new Error("api unavailable"));
    render(createElement(CalculatorPanel));
    await screen.findByLabelText("年利率（%）");
    fillAllFamilyFields("720");
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    expect(await screen.findByText("诊断报告生成失败，请稍后重试。")).toBeInTheDocument();
    expect(screen.queryByText("危险")).not.toBeInTheDocument();
    expect(screen.queryByText("暂缓改善")).not.toBeInTheDocument();
    expect(screen.queryByTestId("pressure-pointer")).not.toBeInTheDocument();
  });

  it("ignores an in-flight response after an input changes", async () => {
    let resolveRequest: (value: CalculationResponse) => void = () => undefined;
    vi.mocked(createCapacityCalculation).mockImplementationOnce(
      () => new Promise((resolve) => { resolveRequest = resolve; }),
    );
    render(createElement(CalculatorPanel));
    await screen.findByLabelText("年利率（%）");
    fillAllFamilyFields();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));
    await waitFor(() => expect(createCapacityCalculation).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("目标总价（万）"), { target: { value: "510" } });
    await act(async () => resolveRequest(reportFixture));

    expect(screen.queryByText("后端返回的完整诊断原因。")).not.toBeInTheDocument();
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
	it("stays locked without requesting private decision data", async () => {
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("出手窗口已锁定")).toBeInTheDocument();
		expect(getActionWindow).not.toHaveBeenCalled();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
	});

	it("renders a loading state without stale recommendation content", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("正在检查出手窗口")).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
	});

	it("does not invent a recommendation when the API is unavailable", async () => {
		setAccessToken("secret-token");
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("决策服务不可用")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
	});

	it.each([
		["capacity_required", "需要换房测算", "/calculator"],
		["watchlist_required", "需要目标小区", "/neighborhoods"],
		["metric_required", "需要市场数据", "/data"],
		["metric_stale", "指标已经过期", "/data"],
		["metric_insufficient", "市场数据不足", "/data"],
	] as const)("renders %s as an independent prerequisite state", async (code, title, href) => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow).mockRejectedValueOnce(new ApiError(code, code, 409));
		render(createElement(ActionWindowPage));

		expect(await screen.findByText(title)).toBeInTheDocument();
		expect(screen.getByRole("link")).toHaveAttribute("href", href);
		expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
		expect(screen.queryByText("行动清单")).not.toBeInTheDocument();
		expect(screen.queryByText("风险警示")).not.toBeInTheDocument();
	});

	it("retries a failed request without retaining the failed state", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow)
			.mockRejectedValueOnce(new Error("offline"))
			.mockResolvedValueOnce({
				action: "等",
				confidence: "中",
				summary: "等待新增数据。",
				checklist: ["复核数据。"],
				risks: ["避免追价。"],
			});
		render(createElement(ActionWindowPage));
		fireEvent.click(await screen.findByRole("button", { name: "重试" }));

		expect(await screen.findByText("建议等")).toBeInTheDocument();
		expect(screen.queryByText("决策服务不可用")).not.toBeInTheDocument();
	});

  it("renders API action window recommendations", async () => {
	setAccessToken("secret-token");
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
	setAccessToken("secret-token");
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
    expect(screen.getByText(/规则版本 2026.07.14/)).toBeInTheDocument();
    expect(screen.getByText("适用范围")).toBeInTheDocument();
    expect(screen.getByText("来源")).toBeInTheDocument();
  });
});

describe("WatchlistPage", () => {
	it("stays locked without requesting private watchlist data", async () => {
		render(createElement(WatchlistPage));

		expect(await screen.findByText("观察池已锁定")).toBeInTheDocument();
		expect(getWatchlist).not.toHaveBeenCalled();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
	});

	it("shows loading without cards or summary values", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(WatchlistPage));

		expect(await screen.findByText("正在加载观察池")).toBeInTheDocument();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
		expect(screen.queryByRole("article")).not.toBeInTheDocument();
	});

	it("does not fall back to sample communities when the API fails", async () => {
		setAccessToken("secret-token");
		render(createElement(WatchlistPage));

		expect(screen.getByText("导出本周报表")).toBeInTheDocument();
		expect(await screen.findByText("观察池读取失败")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
		expect(
			screen.queryByRole("heading", { name: "青枫花园 滨江核心 · 三房" }),
		).not.toBeInTheDocument();
		expect(screen.queryByText(/星河湾/)).not.toBeInTheDocument();
	});

  it("renders API watchlist items when available", async () => {
    setAccessToken("secret-token");
    vi.mocked(getWatchlist).mockResolvedValueOnce({
      items: [
        watchlistFixture({
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
        }),
      ],
    });
    render(createElement(WatchlistPage));

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    expect(screen.getByText("18 套")).toBeInTheDocument();
    expect(
      screen.getByText((content) => content.includes("API 返回的重点建议。")),
    ).toBeInTheDocument();
  });

  it("renders an empty watchlist state for a successful empty API response", async () => {
    setAccessToken("secret-token");
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

	it("renders stale metrics as stale without a buyer-window conclusion", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockResolvedValueOnce({
			items: [
				watchlistFixture({
					name: "过期小区",
					status: "数据不足",
					freshness: "stale",
					qualityState: "low_confidence",
					qualityWarnings: ["stale_data"],
					advice: "等待最新采集批次。",
				}),
			],
		});
		render(createElement(WatchlistPage));

		expect(await screen.findByText("数据已过期")).toBeInTheDocument();
		expect(screen.getByText("过期小区：市场数据已陈旧")).toBeInTheDocument();
		expect(screen.queryByText("适合砍价")).not.toBeInTheDocument();
	});

	it("renders unknown momentum as transaction data insufficient", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockResolvedValueOnce({
			items: [
				watchlistFixture({
					name: "样本不足小区",
					status: "数据不足",
					transactionMomentum: "unknown",
					transactionSampleCount: 1,
					qualityState: "low_confidence",
					qualityWarnings: ["insufficient_transaction_samples"],
					advice: "等待更多成交样本。",
				}),
			],
		});
		render(createElement(WatchlistPage));

		expect(await screen.findByText("成交数据不足")).toBeInTheDocument();
		expect(screen.getByText("样本不足小区：成交样本不足")).toBeInTheDocument();
		expect(screen.queryByText("平稳")).not.toBeInTheDocument();
		expect(screen.queryByText("偏弱")).not.toBeInTheDocument();
	});

	it("retries a failed watchlist request", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist)
			.mockRejectedValueOnce(new Error("offline"))
			.mockResolvedValueOnce({ items: [] });
		render(createElement(WatchlistPage));
		fireEvent.click(await screen.findByRole("button", { name: "重试" }));

		expect(await screen.findByText("观察池暂无小区")).toBeInTheDocument();
		expect(screen.queryByText("观察池读取失败")).not.toBeInTheDocument();
	});
});

function watchlistFixture(overrides: Partial<WatchlistItem> = {}): WatchlistItem {
	return {
		id: "watchlist_1",
		neighborhoodId: "neighborhood_1",
		name: "接口花园",
		area: "南城",
		targetLayout: "两房",
		status: "继续观察",
		listedHomes: 18,
		priceCutHomes: 2,
		transactionMomentum: "stable",
		advice: "继续观察真实数据。",
		hasMetric: true,
		collectionRunId: "11111111-1111-1111-1111-111111111111",
		algorithmVersion: "market-metrics/test.1",
		sourceIds: ["22222222-2222-2222-2222-222222222222"],
		collectedAt: "2026-07-14T08:00:00Z",
		transactionSampleCount: 4,
		coverage: "full",
		freshness: "current",
		qualityState: "sufficient",
		qualityWarnings: [],
		...overrides,
	};
}

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
