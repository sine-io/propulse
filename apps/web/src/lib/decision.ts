export type PressureLevel = "safe" | "strained" | "danger";
export type TransactionMomentum = "weak" | "stable" | "strong";
export type SupplyPressure = "low" | "medium" | "high";
export type NeighborhoodStatus =
  | "重点看"
  | "继续观察"
  | "适合砍价"
  | "价格偏硬"
  | "暂不建议追";
export type ActionWindow = "看" | "等" | "砍价" | "出手";
export type Confidence = "低" | "中" | "高";
export type Scarcity = "low" | "medium" | "high";

export interface NeighborhoodSignalInput {
  name: string;
  listingPriceRange: [number, number];
  transactionPriceRange: [number, number];
  listedHomes: number;
  listedHomesChangePct: number;
  priceCutHomes: number;
  avgDaysOnMarket: number;
  transactionMomentum: TransactionMomentum;
  targetLayoutSupply: number;
}

export interface NeighborhoodSignalResult {
  name: string;
  status: NeighborhoodStatus;
  supplyPressure: SupplyPressure;
  priceCutShare: number;
  priceGapPct: number;
  targetLayoutScarcity: Scarcity;
  nextAction: string;
  reasons: string[];
}

export interface ActionWindowInput {
  budgetPressure: PressureLevel;
  hasDownPaymentGap: boolean;
  neighborhoodStatus: NeighborhoodStatus;
  targetLayoutScarcity: Scarcity;
  alternativesBetter: boolean;
}

export interface ActionWindowResult {
  action: ActionWindow;
  confidence: Confidence;
  summary: string;
  checklist: string[];
  risks: string[];
}

const round = (value: number, digits = 1) => {
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
};

const midpoint = ([min, max]: [number, number]) => (min + max) / 2;

export function evaluateNeighborhoodSignal(
  input: NeighborhoodSignalInput,
): NeighborhoodSignalResult {
  const priceCutShare = input.priceCutHomes / Math.max(input.listedHomes, 1);
  const listingMid = midpoint(input.listingPriceRange);
  const transactionMid = midpoint(input.transactionPriceRange);
  const priceGapPct = (listingMid - transactionMid) / Math.max(listingMid, 1);

  let supplyPressure: SupplyPressure = "medium";
  if (
    input.listedHomesChangePct >= 12 ||
    priceCutShare >= 0.2 ||
    input.avgDaysOnMarket >= 70
  ) {
    supplyPressure = "high";
  } else if (
    input.listedHomes < 20 &&
    input.listedHomesChangePct <= 0 &&
    priceCutShare < 0.1 &&
    input.avgDaysOnMarket < 45
  ) {
    supplyPressure = "low";
  }

  const targetLayoutScarcity: Scarcity =
    input.targetLayoutSupply <= 4
      ? "high"
      : input.targetLayoutSupply <= 10
        ? "medium"
        : "low";

  const reasons: string[] = [];
  if (input.listedHomesChangePct >= 12) {
    reasons.push("挂牌量明显增加，买方可选择空间扩大。");
  }
  if (priceCutShare >= 0.2) {
    reasons.push("降价房源占比超过 20%，房东预期开始松动。");
  }
  if (input.transactionMomentum === "weak") {
    reasons.push("成交偏弱，挂牌价缺少成交支撑。");
  }
  if (supplyPressure === "low") {
    reasons.push("目标户型供给偏少，成交对挂牌价仍有支撑。");
  }

  let status: NeighborhoodStatus = "继续观察";
  let nextAction = "继续每周记录挂牌、降价和成交变化，不急于下判断。";

  if (
    supplyPressure === "high" &&
    priceCutShare >= 0.2 &&
    input.transactionMomentum === "weak"
  ) {
    status = "适合砍价";
    nextAction = `重点看 ${input.transactionPriceRange[0]}-${input.transactionPriceRange[1]} 万成交区间附近房源，对挂牌久、降价过的房源试探底价。`;
  } else if (
    supplyPressure === "low" &&
    input.transactionMomentum === "strong"
  ) {
    status = "价格偏硬";
    nextAction = "不要用单套挂牌价追高，等待新增供应或转向替代小区。";
  } else if (priceGapPct >= 0.05 && input.transactionMomentum !== "strong") {
    status = "重点看";
    nextAction = "可以开始实地看房，记录缺陷并用成交区间校准报价。";
  } else if (priceGapPct < 0.02 && input.transactionMomentum === "weak") {
    status = "暂不建议追";
    nextAction = "价格优势不明显且成交弱，先把预算和替代小区比较清楚。";
  }

  return {
    name: input.name,
    status,
    supplyPressure,
    priceCutShare: round(priceCutShare, 3),
    priceGapPct: round(priceGapPct, 3),
    targetLayoutScarcity,
    nextAction,
    reasons,
  };
}

export function recommendActionWindow(
  input: ActionWindowInput,
): ActionWindowResult {
  if (input.budgetPressure === "danger" || input.hasDownPaymentGap) {
    return {
      action: "等",
      confidence: "高",
      summary:
        "先处理预算与旧房回款，再进入看房或出价动作；否则容易把现金流压到危险区。",
      checklist: [
        "重新测算安全总价，优先消除首付缺口。",
        "推进旧房成交或降低目标总价。",
        "暂停对超预算房源的下定和追价。",
      ],
      risks: ["现金流安全垫不足时，即使小区出现砍价窗口也不宜贸然出手。"],
    };
  }

  if (input.neighborhoodStatus === "适合砍价") {
    return {
      action: "砍价",
      confidence: input.alternativesBetter ? "高" : "中",
      summary:
        "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
      checklist: [
        "约看 3 套成交区间附近、挂牌超过 60 天的目标户型。",
        "用近期成交低位作为报价锚点，先试探 3%-8% 让价空间。",
        "同步比较替代小区，保留不成交也能退出的底气。",
      ],
      risks:
        input.budgetPressure === "strained"
          ? ["预算不是完全宽松，砍价失败时不要上调总价硬追。"]
          : ["单套低价房源可能存在硬伤，不要把个案当成整体价格。"],
    };
  }

  if (
    input.budgetPressure === "safe" &&
    input.neighborhoodStatus === "重点看" &&
    input.targetLayoutScarcity === "high"
  ) {
    return {
      action: "出手",
      confidence: "中",
      summary:
        "预算安全、目标户型稀缺且价格进入可接受区间，可以进入出价准备。",
      checklist: [
        "核验房源硬伤、产权和税费后准备正式报价。",
        "把最高出价锁定在安全总价内。",
        "设置 24 小时冷静复盘，避免被单套稀缺性推着追价。",
      ],
      risks: ["稀缺房源容易造成竞价，必须提前写清最高价和退出条件。"],
    };
  }

  if (input.neighborhoodStatus === "价格偏硬") {
    return {
      action: "等",
      confidence: "中",
      summary: "小区价格仍偏硬，当前重点不是下定，而是等待新增供应或转向替代小区。",
      checklist: [
        "观察未来两周新增挂牌和降价房源是否增加。",
        "把预算相近的替代小区加入观察池。",
      ],
      risks: ["追高会吞掉预算安全垫。"],
    };
  }

  return {
    action: "看",
    confidence: "中",
    summary: "可以开始实地看房，但还不需要急着报价或下定。",
    checklist: [
      "每周记录挂牌、降价、成交和带看反馈。",
      "用看房记录筛掉硬伤房源，沉淀可谈价清单。",
    ],
    risks: ["数据仍不充分，单周变化不能当成明确趋势。"],
  };
}
