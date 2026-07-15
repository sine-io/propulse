import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import RootLayout, { metadata } from "@/app/layout";
import { setAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createReviewNote,
  createCapacityCalculation,
  getActionWindow,
  getCapacityAssumptions,
  getWatchlist,
  listReviewNotes,
  updateReviewNote,
  type ActionWindowResponse,
  type CalculationResponse,
  type CapacityAssumptionsResponse,
  type WatchlistItem,
} from "@/lib/api-client";
import { methodArticles } from "@/lib/method-articles";
import { decisionTemplates } from "@/lib/template-catalog";
import { AppHeader } from "./app-header";
import { ActionWindowPage } from "./action-window-page";
import { CalculatorPanel } from "./calculator-panel";
import { HomePage } from "./home-page";
import { MethodsPage } from "./methods-page";
import { TemplatesPage } from "./templates-page";
import { WatchlistPage } from "./watchlist-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();

  return {
    ...actual,
    createCapacityCalculation: vi.fn(),
    createReviewNote: vi.fn(),
    getActionWindow: vi.fn(),
    getCapacityAssumptions: vi.fn(),
    getWatchlist: vi.fn(),
    listReviewNotes: vi.fn(),
    updateReviewNote: vi.fn(),
  };
});

const actionTargetNeighborhoodID = "11111111-1111-1111-1111-111111111111";
const actionAlternativeNeighborhoodID = "99999999-9999-4999-8999-999999999999";

beforeEach(() => {
	window.sessionStorage.clear();
	window.history.replaceState({}, "", "/");
  vi.mocked(createCapacityCalculation).mockReset();
  vi.mocked(createReviewNote).mockReset();
  vi.mocked(getActionWindow).mockReset();
  vi.mocked(getCapacityAssumptions).mockReset();
  vi.mocked(getWatchlist).mockReset();
  vi.mocked(listReviewNotes).mockReset();
  vi.mocked(updateReviewNote).mockReset();
  vi.mocked(createCapacityCalculation).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(createReviewNote).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getActionWindow).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getCapacityAssumptions).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(getWatchlist).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(listReviewNotes).mockRejectedValue(new Error("api unavailable"));
  vi.mocked(updateReviewNote).mockRejectedValue(new Error("api unavailable"));
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

  it("links each method card to its exact keyboard-reachable article", async () => {
    render(createElement(CalculatorPanel));
	await screen.findByLabelText("年利率（%）");

    const monthlyPaymentLink = screen.getByRole("link", {
      name: /为什么月供安全线比总价更重要/,
    });
    const oldHomeDelayLink = screen.getByRole("link", {
      name: /旧房迟迟卖不掉怎么办/,
    });

    expect(monthlyPaymentLink).toHaveAttribute("href", "/methods/monthly-payment-safety");
    expect(oldHomeDelayLink).toHaveAttribute("href", "/methods/old-home-sale-delay");
    expect(monthlyPaymentLink.tagName).toBe("A");
    expect(oldHomeDelayLink.tagName).toBe("A");
    expect(monthlyPaymentLink.tabIndex).toBe(0);
    expect(oldHomeDelayLink.tabIndex).toBe(0);

    monthlyPaymentLink.focus();
    expect(document.activeElement).toBe(monthlyPaymentLink);
    oldHomeDelayLink.focus();
    expect(document.activeElement).toBe(oldHomeDelayLink);
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

describe("ActionWindowPage", () => {
	beforeEach(() => {
		window.history.replaceState(
			{},
			"",
			`/action-window?neighborhoodId=${actionTargetNeighborhoodID}`,
		);
		vi.mocked(getWatchlist).mockResolvedValue({
			items: [
				watchlistFixture({
					id: "watchlist_action_target",
					neighborhoodId: actionTargetNeighborhoodID,
					name: "接口花园",
					area: "滨江核心",
					targetLayout: "三房",
				}),
				watchlistFixture({
					id: "watchlist_action_alternative",
					neighborhoodId: actionAlternativeNeighborhoodID,
					name: "真实备选花园",
					area: "西城",
					targetLayout: "三房",
				}),
			],
		});
	});

	it("stays locked without requesting private decision data", async () => {
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("出手窗口已锁定")).toBeInTheDocument();
		expect(getWatchlist).not.toHaveBeenCalled();
		expect(getActionWindow).not.toHaveBeenCalled();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
		expect(screen.queryByText("决策因子与证据")).not.toBeInTheDocument();
	});

	it("shows an add-target entry for an empty watchlist without requesting a recommendation", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockResolvedValueOnce({ items: [] });
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("观察池暂无小区")).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "添加目标小区" })).toHaveAttribute("href", "/neighborhoods");
		expect(getActionWindow).not.toHaveBeenCalled();
	});

	it("requires an explicit first selection even when watched neighborhoods exist", async () => {
		setAccessToken("secret-token");
		window.history.replaceState({}, "", "/action-window");
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("选择目标小区")).toBeInTheDocument();
		expect(screen.getByRole("combobox", { name: "目标小区" })).toHaveValue("");
		expect(getActionWindow).not.toHaveBeenCalled();
	});

	it("persists a selection in the URL and requests that neighborhood", async () => {
		setAccessToken("secret-token");
		window.history.replaceState({}, "", "/action-window");
		vi.mocked(getActionWindow).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(ActionWindowPage));

		const selector = await screen.findByRole("combobox", { name: "目标小区" });
		fireEvent.change(selector, { target: { value: actionAlternativeNeighborhoodID } });

		await waitFor(() => {
			expect(getActionWindow).toHaveBeenCalledWith(
				actionAlternativeNeighborhoodID,
				expect.any(AbortSignal),
			);
		});
		expect(window.location.search).toBe(`?neighborhoodId=${actionAlternativeNeighborhoodID}`);
		expect(await screen.findByText("正在检查出手窗口")).toBeInTheDocument();
	});

	it("restores a valid URL selection on refresh", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(ActionWindowPage));

		expect(await screen.findByRole("combobox", { name: "目标小区" })).toHaveValue(actionTargetNeighborhoodID);
		await waitFor(() => expect(getActionWindow).toHaveBeenCalledWith(
			actionTargetNeighborhoodID,
			expect.any(AbortSignal),
		));
	});

	it("requires reselection when the URL target is no longer watched", async () => {
		setAccessToken("secret-token");
		window.history.replaceState(
			{},
			"",
			"/action-window?neighborhoodId=aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		);
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("目标已不在观察池")).toBeInTheDocument();
		expect(screen.getByRole("combobox", { name: "目标小区" })).toHaveValue("");
		expect(getActionWindow).not.toHaveBeenCalled();
	});

	it("renders a loading state without stale recommendation content", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("正在检查出手窗口")).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
		expect(screen.queryByText("决策因子与证据")).not.toBeInTheDocument();
	});

	it("does not invent a recommendation when the API is unavailable", async () => {
		setAccessToken("secret-token");
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("决策服务不可用")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
		expect(screen.queryByText("当前核心策略")).not.toBeInTheDocument();
		expect(screen.queryByText("决策因子与证据")).not.toBeInTheDocument();
	});

	it.each([
		["capacity_required", "需要换房测算", "/calculator"],
		["watchlist_required", "需要目标小区", "/watchlist"],
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
		expect(screen.queryByText("决策因子与证据")).not.toBeInTheDocument();
		expect(screen.queryByText("行动清单")).not.toBeInTheDocument();
		expect(screen.queryByText("风险警示")).not.toBeInTheDocument();
	});

	it("retries a failed request without retaining the failed state", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow)
			.mockRejectedValueOnce(new Error("offline"))
			.mockResolvedValueOnce(actionWindowFixture({
				action: "等",
				confidence: "中",
				summary: "等待新增数据。",
				checklist: ["复核数据。"],
				risks: ["避免追价。"],
			}));
		render(createElement(ActionWindowPage));
		fireEvent.click(await screen.findByRole("button", { name: "重试" }));

		expect(await screen.findByText("建议等")).toBeInTheDocument();
		expect(screen.queryByText("决策服务不可用")).not.toBeInTheDocument();
	});

	it("clears the previous recommendation and checklist while switching neighborhoods", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow)
			.mockResolvedValueOnce(actionWindowFixture({
				summary: "第一小区建议仍在展示。",
				checklist: ["第一小区核验项。"],
			}))
			.mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("第一小区核验项。")).toBeInTheDocument();

		fireEvent.change(screen.getByRole("combobox", { name: "目标小区" }), {
			target: { value: actionAlternativeNeighborhoodID },
		});

		expect(await screen.findByText("正在检查出手窗口")).toBeInTheDocument();
		expect(screen.queryByText("第一小区建议仍在展示。")).not.toBeInTheDocument();
		expect(screen.queryByText("第一小区核验项。")).not.toBeInTheDocument();
		await waitFor(() => expect(getActionWindow).toHaveBeenLastCalledWith(
			actionAlternativeNeighborhoodID,
			expect.any(AbortSignal),
		));
	});

	it("follows browser history selections without retaining the previous target", async () => {
		setAccessToken("secret-token");
		vi.mocked(getActionWindow)
			.mockResolvedValueOnce(actionWindowFixture({ summary: "第一目标建议。" }))
			.mockResolvedValueOnce(actionWindowFixture({
				summary: "第二目标建议。",
				target: {
					neighborhoodId: actionAlternativeNeighborhoodID,
					name: "真实备选花园",
					area: "西城",
					targetLayout: "三房",
				},
			}))
			.mockResolvedValueOnce(actionWindowFixture({ summary: "返回第一目标建议。" }));
		render(createElement(ActionWindowPage));

		expect(await screen.findByText("第一目标建议。")).toBeInTheDocument();
		act(() => {
			window.history.pushState(
				{},
				"",
				`/action-window?neighborhoodId=${actionAlternativeNeighborhoodID}`,
			);
			window.dispatchEvent(new PopStateEvent("popstate"));
		});
		expect(await screen.findByText("第二目标建议。")).toBeInTheDocument();
		expect(screen.queryByText("第一目标建议。")).not.toBeInTheDocument();

		act(() => {
			window.history.replaceState(
				{},
				"",
				`/action-window?neighborhoodId=${actionTargetNeighborhoodID}`,
			);
			window.dispatchEvent(new PopStateEvent("popstate"));
		});
		expect(await screen.findByText("返回第一目标建议。")).toBeInTheDocument();
		expect(getActionWindow).toHaveBeenLastCalledWith(
			actionTargetNeighborhoodID,
			expect.any(AbortSignal),
		);
	});

  it("renders API action window recommendations", async () => {
	setAccessToken("secret-token");
    vi.mocked(getActionWindow).mockResolvedValueOnce(actionWindowFixture({
      action: "出手",
      confidence: "高",
      confidenceReasons: ["目标小区支持议价，且版本化比较发现至少一个预算内更优备选。"],
      summary: "预算安全且目标户型稀缺，可以准备出手。",
      checklist: ["确认贷款批复。", "准备谈价底线。"],
      risks: ["不要因为稀缺而突破预算。"],
      alternativeComparison: betterAlternativeComparisonFixture(),
      factors: actionWindowFixture().factors.map((factor) =>
        factor.key === "market_signal"
          ? { ...factor, summary: "API 指标显示目标小区进入可谈区间。" }
          : factor.key === "alternatives"
            ? {
                ...factor,
                status: "positive",
                summary: "发现 1 个满足版本化规则的更优备选。",
                evidence: [
                  { key: "better_candidate_count", label: "更优候选", valueType: "number", numberValue: 1, unit: "个" },
                ],
              }
          : factor,
      ),
    }));
    render(createElement(ActionWindowPage));

    expect(await screen.findByText("建议出手")).toBeInTheDocument();
    expect(screen.getByText("高")).toBeInTheDocument();
    expect(screen.getByText("接口花园")).toBeInTheDocument();
    expect(screen.getByText("API 指标显示目标小区进入可谈区间。")).toBeInTheDocument();
    expect(screen.getByText("510 万元")).toBeInTheDocument();
    expect(screen.getByText("发现 1 个满足版本化规则的更优备选。")).toBeInTheDocument();
    expect(screen.getByText("真实备选花园")).toBeInTheDocument();
    expect(screen.getByText("500 → 450 万元（-50 / -10%）")).toBeInTheDocument();
    expect(screen.getByText("8 → 10 套（+2 / +25%）")).toBeInTheDocument();
    expect(screen.getByText("预算内、至少两项改善且无劣化")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /接口花园/ })).toHaveAttribute(
      "href",
      "/neighborhoods?id=11111111-1111-1111-1111-111111111111",
    );
    expect(screen.getAllByRole("link", { name: /资金测算记录/ })[0]).toHaveAttribute(
      "href",
      "/calculator?calculationId=22222222-2222-2222-2222-222222222222",
    );
    expect(screen.getAllByRole("link", { name: /小区指标批次/ })[0]).toHaveAttribute(
      "href",
      "/data/imports/44444444-4444-4444-4444-444444444444",
    );
    expect(screen.getByRole("link", { name: /候选批次/ })).toHaveAttribute(
      "href",
      "/data/imports/88888888-8888-4888-8888-888888888888",
    );
    expect(screen.queryByText(/同户型约 12 套/)).not.toBeInTheDocument();
    expect(screen.queryByText(/已添加 2 个/)).not.toBeInTheDocument();
    expect(screen.getByText("确认贷款批复。")).toBeInTheDocument();
    expect(screen.getByText("不要因为稀缺而突破预算。")).toBeInTheDocument();
		expect(screen.getByRole("heading", { name: "接口花园出手窗口" })).toBeInTheDocument();
		expect(screen.getByRole("option", { name: "接口花园 · 三房", selected: true })).toBeInTheDocument();
		expect(screen.getAllByText(/指标采集/).length).toBeGreaterThanOrEqual(2);
		expect(screen.getByText(/挂牌 42 \/ 成交 5/)).toHaveTextContent("2026");
		expect(getActionWindow).toHaveBeenCalledWith(
			actionTargetNeighborhoodID,
			expect.any(AbortSignal),
		);
  });

  it("renders an unknown alternative without inventing comparison values", async () => {
    setAccessToken("secret-token");
    const unknownComparison: ActionWindowResponse["alternativeComparison"] = {
      status: "unknown",
      ruleVersion: "alternative-comparison/2026.07.14.1",
      referenceCollectedAt: "2026-07-14T08:00:00Z",
      safeTotalPrice: 510,
      candidates: [
        {
          neighborhoodId: "99999999-9999-4999-8999-999999999999",
          name: "缺指标候选",
          area: "西城",
          targetLayout: "三房",
          status: "unknown",
          reasons: ["metric_missing"],
          improvements: [],
          deteriorations: [],
          withinBudget: null,
          targetTransactionPriceMidpoint: null,
          candidateTransactionPriceMidpoint: null,
          priceDifference: null,
          priceDifferencePct: null,
          targetSignal: null,
          candidateSignal: null,
          signalRankDifference: null,
          targetLayoutSupply: 8,
          candidateTargetLayoutSupply: null,
          supplyDifference: null,
          supplyDifferencePct: null,
          metric: null,
        },
      ],
    };
    vi.mocked(getActionWindow).mockResolvedValueOnce(actionWindowFixture({
      alternativeComparison: unknownComparison,
      factors: actionWindowFixture().factors.map((factor) =>
        factor.key === "alternatives"
          ? { ...factor, status: "unknown", summary: "备选数据不足，无法判断是否更优。" }
          : factor,
      ),
    }));
    render(createElement(ActionWindowPage));

    expect(await screen.findByText("缺指标候选")).toBeInTheDocument();
    expect(screen.getByText("缺少当前算法指标")).toBeInTheDocument();
    expect(screen.getAllByText("不可比").length).toBeGreaterThanOrEqual(3);
    expect(screen.getByText("无合格指标来源")).toBeInTheDocument();
    expect(screen.queryByText("0 万元")).not.toBeInTheDocument();
  });

  it("renders the action checklist as a non-interactive list (ACTION-005)", async () => {
	setAccessToken("secret-token");
    vi.mocked(getActionWindow).mockResolvedValueOnce(actionWindowFixture({
      action: "出手",
      confidence: "高",
      summary: "预算安全且目标户型稀缺，可以准备出手。",
      checklist: ["确认贷款批复。", "准备谈价底线。"],
      risks: ["不要因为稀缺而突破预算。"],
    }));
    render(createElement(ActionWindowPage));

    const checklist = await screen.findByRole("list", { name: "行动清单" });
    const items = within(checklist).getAllByRole("listitem");

    expect(items).toHaveLength(2);
    expect(within(checklist).getByText("确认贷款批复。")).toBeInTheDocument();
    expect(within(checklist).getByText("准备谈价底线。")).toBeInTheDocument();
    expect(within(checklist).queryByRole("checkbox")).not.toBeInTheDocument();
    expect(within(checklist).queryByRole("button")).not.toBeInTheDocument();
    for (const item of items) {
      expect(item).not.toHaveAttribute("tabindex");
      expect(item).not.toHaveAttribute("role", "button");
      expect(item.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    }
  });
});

function actionWindowFixture(overrides: Partial<ActionWindowResponse> = {}): ActionWindowResponse {
  const capacitySource = {
    type: "capacity_calculation" as const,
    id: "22222222-2222-2222-2222-222222222222",
    observedAt: "2026-07-14T07:30:00Z",
  };
  const metricSource = {
    type: "neighborhood_metric" as const,
    id: "33333333-3333-3333-3333-333333333333",
    observedAt: "2026-07-14T08:00:00Z",
  };
  const alternativeSource = {
    type: "alternative_comparison" as const,
    id: "alternative-comparison/2026.07.14.1",
    observedAt: "2026-07-14T08:00:00Z",
  };
  return {
    action: "砍价",
    confidence: "中",
    confidenceReasons: ["目标小区支持议价，但备选比较没有发现满足规则的更优候选。"],
    summary: "预算与当前市场证据支持试探底价。",
    target: {
      neighborhoodId: "11111111-1111-1111-1111-111111111111",
      name: "接口花园",
      area: "滨江核心",
      targetLayout: "三房",
    },
    capacityCalculation: {
      id: capacitySource.id,
      createdAt: capacitySource.observedAt,
      ruleVersion: "capacity/2026.07.14.1",
      traceabilityStatus: "complete",
    },
    metric: {
      id: metricSource.id,
      collectionRunId: "44444444-4444-4444-4444-444444444444",
      algorithmVersion: "market-metrics/2026.07.14.1",
      collectedAt: metricSource.observedAt,
      calculatedAt: "2026-07-14T08:05:00Z",
      sourceIds: ["55555555-5555-5555-5555-555555555555"],
      listingSampleCount: 42,
      transactionSampleCount: 5,
      coverage: "full",
      freshness: "current",
      qualityState: "sufficient",
      qualityWarnings: [],
    },
    alternativeComparison: {
      status: "none",
      ruleVersion: alternativeSource.id,
      referenceCollectedAt: alternativeSource.observedAt,
      safeTotalPrice: 510,
      candidates: [],
    },
    factors: [
      {
        key: "budget_pressure",
        status: "positive",
        summary: "资金压力处于安全区。",
        source: capacitySource,
        evidence: [
          { key: "safe_total_price", label: "安全总价", valueType: "number", numberValue: 510, unit: "万元" },
        ],
      },
      {
        key: "down_payment_gap",
        status: "positive",
        summary: "当前测算没有首付缺口。",
        source: capacitySource,
        evidence: [
          { key: "has_down_payment_gap", label: "存在首付缺口", valueType: "boolean", booleanValue: false },
        ],
      },
      {
        key: "market_signal",
        status: "positive",
        summary: "目标小区信号支持议价。",
        source: metricSource,
        evidence: [
          { key: "neighborhood_status", label: "小区信号", valueType: "text", textValue: "适合砍价" },
        ],
      },
      {
        key: "transaction_momentum",
        status: "positive",
        summary: "真实成交动量偏弱，买方议价条件相对有利。",
        source: metricSource,
        evidence: [
          { key: "recent_30_day_count", label: "近 30 天成交", valueType: "number", numberValue: 1, unit: "笔" },
        ],
      },
      {
        key: "target_layout_supply",
        status: "neutral",
        summary: "目标户型当前供给 8 套，稀缺度为中。",
        source: metricSource,
        evidence: [
          { key: "target_layout_supply", label: "目标户型供给", valueType: "number", numberValue: 8, unit: "套" },
        ],
      },
      {
        key: "alternatives",
        status: "neutral",
        summary: "观察池中没有其他可比较小区。",
        source: alternativeSource,
        evidence: [
          { key: "comparison_status", label: "比较结果", valueType: "text", textValue: "none" },
        ],
      },
    ],
    checklist: ["核验目标房源。"],
    risks: ["不要突破安全总价。"],
    ...overrides,
  };
}

function betterAlternativeComparisonFixture(): ActionWindowResponse["alternativeComparison"] {
  return {
    status: "better_found",
    ruleVersion: "alternative-comparison/2026.07.14.1",
    referenceCollectedAt: "2026-07-14T08:00:00Z",
    safeTotalPrice: 510,
    candidates: [
      {
        neighborhoodId: "66666666-6666-4666-8666-666666666666",
        name: "真实备选花园",
        area: "南城",
        targetLayout: "三房",
        status: "better",
        reasons: ["better_threshold_met"],
        improvements: ["transaction_price", "target_layout_supply"],
        deteriorations: [],
        withinBudget: true,
        targetTransactionPriceMidpoint: 500,
        candidateTransactionPriceMidpoint: 450,
        priceDifference: -50,
        priceDifferencePct: -10,
        targetSignal: "适合砍价",
        candidateSignal: "适合砍价",
        signalRankDifference: 0,
        targetLayoutSupply: 8,
        candidateTargetLayoutSupply: 10,
        supplyDifference: 2,
        supplyDifferencePct: 25,
        metric: {
          id: "77777777-7777-4777-8777-777777777777",
          collectionRunId: "88888888-8888-4888-8888-888888888888",
          algorithmVersion: "market-metrics/2026.07.14.1",
          collectedAt: "2026-07-13T08:00:00Z",
          calculatedAt: "2026-07-13T08:05:00Z",
          sourceIds: [],
          listingSampleCount: 20,
          transactionSampleCount: 3,
          coverage: "full",
          freshness: "current",
          qualityState: "sufficient",
          qualityWarnings: [],
        },
      },
    ],
  };
}

describe("MethodsPage", () => {
  it("renders the default article with the complete methodology structure", () => {
    render(createElement(MethodsPage));

    expect(screen.getByText("问题场景目录")).toBeInTheDocument();
    expect(screen.getByText("真实问题")).toBeInTheDocument();
    expect(screen.getByText("常见误判")).toBeInTheDocument();
    expect(screen.getByText("正确判断")).toBeInTheDocument();
    expect(screen.getByText("你需要盯住的关键指标")).toBeInTheDocument();
    expect(screen.getByText("示例（仅用于说明判断过程）")).toBeInTheDocument();
    expect(screen.getByText("行动建议")).toBeInTheDocument();
    expect(screen.getByText("前往目标小区实践")).toBeInTheDocument();
  });

  it("uses real article links and marks only the default article as current", () => {
    render(createElement(MethodsPage));

    for (const [index, article] of methodArticles.entries()) {
      const link = screen.getByRole("link", { name: article.title });
      expect(link).toHaveAttribute("href", `/methods/${article.slug}`);
      if (index === 0) {
        expect(link).toHaveAttribute("aria-current", "page");
      } else {
        expect(link).not.toHaveAttribute("aria-current");
      }
    }
    expect(screen.queryByText("即将上线")).not.toBeInTheDocument();
  });

  it("keeps the selected article, heading, body, and active navigation in sync", () => {
    for (const article of methodArticles) {
      const view = render(createElement(MethodsPage, { article }));

      expect(screen.getByRole("heading", { level: 2, name: article.title })).toBeInTheDocument();
      expect(screen.getByText(article.realQuestion)).toBeInTheDocument();
      expect(screen.getByRole("link", { name: article.title })).toHaveAttribute(
        "aria-current",
        "page",
      );
      expect(screen.getByRole("link", { name: article.title })).toHaveAttribute(
        "href",
        `/methods/${article.slug}`,
      );

      view.unmount();
    }
  });

  it("shows rule provenance without fixed price or bargaining promises", () => {
    render(createElement(MethodsPage));

    expect(screen.queryByText(/超过 20%/)).not.toBeInTheDocument();
    expect(screen.queryByText(/砍 3%-5%/)).not.toBeInTheDocument();
    expect(screen.getByText("方法适用范围与来源")).toBeInTheDocument();
    expect(screen.getByText(/规则版本 2026.07.14/)).toBeInTheDocument();
    expect(screen.getByText("本文适用范围")).toBeInTheDocument();
    expect(screen.getByText("规则适用范围")).toBeInTheDocument();
    expect(screen.getByText("来源")).toBeInTheDocument();
  });
});

describe("WatchlistPage", () => {
	it("stays locked without requesting private watchlist data", async () => {
		render(createElement(WatchlistPage));

		expect(await screen.findByText("观察池已锁定")).toBeInTheDocument();
		expect(getWatchlist).not.toHaveBeenCalled();
		expect(screen.getByRole("button", { name: "解锁后可导出" })).toBeDisabled();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
	});

	it("shows loading without cards or summary values", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockReturnValueOnce(new Promise(() => undefined));
		render(createElement(WatchlistPage));

		expect(await screen.findByText("正在加载观察池")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "观察池加载中" })).toBeDisabled();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
		expect(screen.queryByRole("article")).not.toBeInTheDocument();
	});

	it("does not fall back to sample communities when the API fails", async () => {
		setAccessToken("secret-token");
		render(createElement(WatchlistPage));

		expect(await screen.findByText("观察池读取失败")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "数据读取失败" })).toBeDisabled();
		expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
		expect(screen.queryByText("观察小区")).not.toBeInTheDocument();
		expect(
			screen.queryByRole("heading", { name: "青枫花园 滨江核心 · 三房" }),
		).not.toBeInTheDocument();
		expect(screen.queryByText(/星河湾/)).not.toBeInTheDocument();
	});

  it("renders API watchlist items when available", async () => {
    setAccessToken("secret-token");
    const downloadReport = vi.fn(() => "propulse-watchlist-2026-07-13.csv");
    const item = watchlistFixture({
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
    });
    vi.mocked(getWatchlist).mockResolvedValueOnce({
      items: [item],
    });
    render(<WatchlistPage downloadReport={downloadReport} />);

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    expect(screen.getByText("18 套")).toBeInTheDocument();
    expect(
      screen.getByText((content) => content.includes("API 返回的重点建议。")),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "导出本周 CSV" }));
    expect(downloadReport).toHaveBeenCalledWith([item]);
    expect(screen.getByText("propulse-watchlist-2026-07-13.csv 已开始下载。")).toBeInTheDocument();
  });

  it("shows a visible error when CSV download setup fails", async () => {
    setAccessToken("secret-token");
    vi.mocked(getWatchlist).mockResolvedValueOnce({ items: [watchlistFixture()] });
    render(<WatchlistPage downloadReport={() => { throw new Error("blob unavailable"); }} />);

    fireEvent.click(await screen.findByRole("button", { name: "导出本周 CSV" }));
    expect(screen.getByRole("alert")).toHaveTextContent("CSV 导出失败，请重试。");
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

    expect(screen.getAllByText("0")).toHaveLength(5);
    expect(screen.getByText("观察池暂无小区")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "暂无数据可导出" })).toBeDisabled();
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

		expect(await screen.findByText("数据已陈旧")).toBeInTheDocument();
		expect(screen.getByText("过期小区：市场数据已陈旧")).toBeInTheDocument();
		expect(screen.queryByText("适合砍价")).not.toBeInTheDocument();
	});

	it("renders weekly comparison values and their source batches", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockResolvedValueOnce({
			items: [
				watchlistFixture({
					name: "周对比小区",
					weeklyComparison: {
						status: "available",
						currentBatch: {
							collectionRunId: "11111111-1111-1111-1111-111111111111",
							dataSourceId: "22222222-2222-2222-2222-222222222222",
							sourceRef: "current.csv",
							collectedAt: "2026-07-14T08:00:00Z",
							coverage: "full",
						},
						baselineBatch: {
							collectionRunId: "33333333-3333-3333-3333-333333333333",
							dataSourceId: "22222222-2222-2222-2222-222222222222",
							sourceRef: "baseline.csv",
							collectedAt: "2026-07-07T08:00:00Z",
							coverage: "full",
						},
						listedHomes: metricChange(18, 15),
						priceCutHomes: metricChange(6, 4),
						recent30DayTransactions: metricChange(5, 5),
					},
				}),
			],
		});
		render(createElement(WatchlistPage));

		expect(await screen.findByText("周对比小区")).toBeInTheDocument();
		expect(screen.getByText("+3（+20.0%）")).toBeInTheDocument();
		expect(screen.getByText("+2（+50.0%）")).toBeInTheDocument();
		expect(screen.getByText("0（0.0%）")).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "当前批次" })).toHaveAttribute(
			"href",
			"/data/imports/11111111-1111-1111-1111-111111111111",
		);
		expect(screen.getByRole("link", { name: "基准批次" })).toHaveAttribute(
			"href",
			"/data/imports/33333333-3333-3333-3333-333333333333",
		);
	});

	it("explains when no full weekly baseline is available", async () => {
		setAccessToken("secret-token");
		vi.mocked(getWatchlist).mockResolvedValueOnce({
			items: [
				watchlistFixture({
					name: "无周基线小区",
					weeklyComparison: {
						status: "unavailable",
						reason: "full_baseline_not_found",
						currentBatch: {
							collectionRunId: "11111111-1111-1111-1111-111111111111",
							dataSourceId: "22222222-2222-2222-2222-222222222222",
							sourceRef: "current.csv",
							collectedAt: "2026-07-14T08:00:00Z",
							coverage: "full",
						},
					},
				}),
			],
		});
		render(createElement(WatchlistPage));

		expect(
			await screen.findByText("暂无本周对比：基准窗口内没有完整批次"),
		).toBeInTheDocument();
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
		city: "杭州",
		area: "南城",
		targetLayout: "两房",
		status: "继续观察",
		listedHomes: 18,
		priceCutHomes: 2,
		transactionMomentum: "stable",
		targetLayoutSupply: 6,
		targetLayoutScarcity: "medium",
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
		weeklyComparison: null,
		...overrides,
	};
}

function metricChange(current: number, baseline: number) {
	return {
		current,
		baseline,
		absoluteChange: current - baseline,
		percentageChange: ((current - baseline) / baseline) * 100,
		percentageStatus: "available" as const,
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

  it("copies every complete, versioned template to the clipboard (TEMPLATE-001)", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(createElement(TemplatesPage));

    for (const [index, template] of decisionTemplates.entries()) {
      const card = screen.getByRole("heading", { name: template.title }).closest("article");
      expect(card).not.toBeNull();
      fireEvent.click(within(card!).getByRole("button", { name: "复制模板结构" }));
      await waitFor(() => expect(writeText).toHaveBeenCalledTimes(index + 1));
      const copied = writeText.mock.calls[index]?.[0] as string;
      expect(copied).toContain(`propulse-template id="${template.id}" version="${template.version}"`);
      expect(copied).toContain(`## ${template.sections[0].title}`);
      expect(copied).toContain(`- ${template.sections.at(-1)?.fields.at(-1)}：`);
      expect(within(card!).getByRole("status")).toHaveTextContent(
        `${template.title} ${template.version} 已复制到剪贴板。`,
      );
    }

    vi.unstubAllGlobals();
  });

  it("announces clipboard failures accessibly", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("permission denied"));
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(createElement(TemplatesPage));

    fireEvent.click(screen.getAllByRole("button", { name: "复制模板结构" })[0]);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "换房预算表 复制失败，请检查剪贴板权限后重试。",
    );
    vi.unstubAllGlobals();
  });
});
