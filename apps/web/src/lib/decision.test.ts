import { describe, expect, it } from "vitest";

import {
  evaluateNeighborhoodSignal,
  recommendActionWindow,
} from "./decision";

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
