import type {
  ActionWindowInput,
  HousingCapacityInput,
  NeighborhoodSignalInput,
} from "./decision";

export const defaultHousingInput: HousingCapacityInput = {
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
};

export const defaultNeighborhoodInput: NeighborhoodSignalInput = {
  name: "青枫花园",
  listingPriceRange: [520, 620],
  transactionPriceRange: [495, 545],
  listedHomes: 42,
  listedHomesChangePct: 18,
  priceCutHomes: 11,
  avgDaysOnMarket: 78,
  transactionMomentum: "weak",
  targetLayoutSupply: 12,
};

export const actionWindowInput: ActionWindowInput = {
  budgetPressure: "strained",
  hasDownPaymentGap: false,
  neighborhoodStatus: "适合砍价",
  targetLayoutScarcity: "medium",
  alternativesBetter: true,
};

export const alternateNeighborhoods = [
  {
    name: "云澜府",
    area: "城东新区",
    layout: "四房 120-140m2",
    status: "继续观察",
    listedHomes: 28,
    priceCutHomes: 2,
    transaction: "平稳",
    advice: "挂牌价无明显松动，且超出安全预算，建议暂缓约看。",
  },
  {
    name: "星河湾",
    area: "滨江外溢",
    layout: "三房 95-105m2",
    status: "重点看",
    listedHomes: 35,
    priceCutHomes: 7,
    transaction: "偏弱",
    advice: "本周新增一套低位挂牌，可加入备选比较。",
  },
];

export const methodTopics = [
  {
    title: "挂牌变多但成交弱，说明什么？",
    wrong: "只看挂牌量增加，就判断房价一定马上大跌。",
    right:
      "挂牌增加但成交没有同步放大，通常说明库存积压和房东竞争加剧，买方议价空间正在打开。",
    action: "开始看房，不急下定，优先砍挂牌久、有降价记录的房源。",
  },
  {
    title: "为什么不能只看挂牌价？",
    wrong: "把房东报价当成真实市场价格。",
    right:
      "挂牌价代表卖方预期，成交价才是市场认可；挂牌成交差越大，越要用成交低位校准报价。",
    action: "每周记录挂牌均价、成交区间和降价比例。",
  },
  {
    title: "为什么月供安全线比总价重要？",
    wrong: "只看能贷多少，不看家庭现金流。",
    right:
      "总价只是入口，月供收入比决定换房后能否长期稳定持有。",
    action: "把目标总价压回安全月供线，再讨论是否出手。",
  },
];

export const templates = [
  {
    title: "换房预算表",
    description: "汇总现金、旧房净回款、税费、装修和过渡成本。",
    fields: ["现金", "旧房底价", "贷款余额", "安全总价"],
  },
  {
    title: "目标小区观察表",
    description: "每周记录小区挂牌、成交、降价和供应压力。",
    fields: ["挂牌量", "成交区间", "降价套数", "成交周期"],
  },
  {
    title: "周监测表",
    description: "把 3-10 个关注小区放在同一张表中横向比较。",
    fields: ["本周变化", "异常信号", "行动建议", "下周重点"],
  },
  {
    title: "看房记录表",
    description: "记录户型、楼层、硬伤、报价和可砍价依据。",
    fields: ["房源信息", "优点", "硬伤", "报价锚点"],
  },
  {
    title: "谈价清单",
    description: "看房前准备可以用于议价的事实和退出条件。",
    fields: ["近期成交", "挂牌时长", "降价记录", "最高出价"],
  },
  {
    title: "决策复盘表",
    description: "复盘本周看房、预算变化、目标变化和最终行动。",
    fields: ["做了什么", "看到什么", "调整什么", "下周动作"],
  },
];
