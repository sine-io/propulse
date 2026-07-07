import { describe, expect, it } from "vitest";

import {
  calculateHousingCapacity,
  evaluateNeighborhoodSignal,
  recommendActionWindow,
} from "./decision";

describe("calculateHousingCapacity", () => {
  it("classifies a target price as strained when monthly pressure is above the safe line but still serviceable", () => {
    const result = calculateHousingCapacity({
      cashOnHand: 150,
      oldHomeValue: 320,
      oldLoanBalance: 80,
      monthlyIncome: 3.5,
      currentMonthlyMortgage: 0,
      acceptableMonthlyMortgage: 1.5,
      targetTotalPrice: 550,
      renovationBudget: 40,
      transactionCosts: 18,
      transitionRentCost: 5,
    });

    expect(result.netOldHomeProceeds).toBe(240);
    expect(result.pressureLevel).toBe("strained");
    expect(result.safeTotalPrice).toBeGreaterThan(500);
    expect(result.safeTotalPrice).toBeLessThan(result.strainedTotalPrice);
    expect(result.dangerTotalPrice).toBeGreaterThan(result.strainedTotalPrice);
    expect(result.monthlyPaymentRatio).toBeGreaterThan(0.35);
    expect(result.monthlyPaymentRatio).toBeLessThanOrEqual(0.45);
    expect(result.strategy).toBe("先卖后买或同步推进");
    expect(result.reasons).toContain("旧房净回款占首付能力比重较高，未锁定成交前不宜贸然下定。");
  });

  it("marks the plan dangerous when the target monthly payment would exceed the cash-flow danger line", () => {
    const result = calculateHousingCapacity({
      cashOnHand: 80,
      oldHomeValue: 260,
      oldLoanBalance: 140,
      monthlyIncome: 2.4,
      currentMonthlyMortgage: 0.35,
      acceptableMonthlyMortgage: 1.4,
      targetTotalPrice: 650,
      renovationBudget: 35,
      transactionCosts: 22,
      transitionRentCost: 8,
    });

    expect(result.pressureLevel).toBe("danger");
    expect(result.downPaymentGap).toBeGreaterThan(0);
    expect(result.strategy).toBe("暂缓改善");
    expect(result.reasons).toContain("目标总价对应的月供收入比超过危险线，现金流缓冲不足。");
  });
});

describe("evaluateNeighborhoodSignal", () => {
  it("opens a bargaining window when supply and price cuts rise while transactions stay weak", () => {
    const result = evaluateNeighborhoodSignal({
      name: "青枫花园",
      listingPriceRange: [520, 620],
      transactionPriceRange: [495, 545],
      listedHomes: 42,
      listedHomesChangePct: 18,
      priceCutHomes: 11,
      avgDaysOnMarket: 78,
      transactionMomentum: "weak",
      targetLayoutSupply: 12,
    });

    expect(result.status).toBe("适合砍价");
    expect(result.supplyPressure).toBe("high");
    expect(result.priceGapPct).toBeGreaterThan(0.08);
    expect(result.reasons).toEqual(
      expect.arrayContaining([
        "挂牌量明显增加，买方可选择空间扩大。",
        "降价房源占比超过 20%，房东预期开始松动。",
        "成交偏弱，挂牌价缺少成交支撑。",
      ]),
    );
  });

  it("keeps the status hard when supply is tight and transactions support current prices", () => {
    const result = evaluateNeighborhoodSignal({
      name: "云澜府",
      listingPriceRange: [700, 760],
      transactionPriceRange: [690, 745],
      listedHomes: 14,
      listedHomesChangePct: -6,
      priceCutHomes: 1,
      avgDaysOnMarket: 35,
      transactionMomentum: "strong",
      targetLayoutSupply: 3,
    });

    expect(result.status).toBe("价格偏硬");
    expect(result.supplyPressure).toBe("low");
    expect(result.nextAction).toContain("不要用单套挂牌价追高");
  });
});

describe("recommendActionWindow", () => {
  it("recommends bargaining when budget is serviceable and the target community has a buyer window", () => {
    const result = recommendActionWindow({
      budgetPressure: "strained",
      hasDownPaymentGap: false,
      neighborhoodStatus: "适合砍价",
      targetLayoutScarcity: "medium",
      alternativesBetter: true,
    });

    expect(result.action).toBe("砍价");
    expect(result.confidence).toBe("高");
    expect(result.checklist[0]).toContain("约看");
    expect(result.risks).toContain("预算不是完全宽松，砍价失败时不要上调总价硬追。");
  });

  it("recommends waiting when either budget pressure or a funding gap makes the move unsafe", () => {
    const result = recommendActionWindow({
      budgetPressure: "danger",
      hasDownPaymentGap: true,
      neighborhoodStatus: "适合砍价",
      targetLayoutScarcity: "low",
      alternativesBetter: false,
    });

    expect(result.action).toBe("等");
    expect(result.confidence).toBe("高");
    expect(result.summary).toContain("先处理预算与旧房回款");
  });

  it("recommends acting when budget is safe, price is fair, and the target layout is scarce", () => {
    const result = recommendActionWindow({
      budgetPressure: "safe",
      hasDownPaymentGap: false,
      neighborhoodStatus: "重点看",
      targetLayoutScarcity: "high",
      alternativesBetter: false,
    });

    expect(result.action).toBe("出手");
    expect(result.confidence).toBe("中");
    expect(result.summary).toContain("可以进入出价准备");
  });
});
