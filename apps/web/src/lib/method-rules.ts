// 方法规则的版本与来源元数据（METHOD-002 / #33）。
//
// 方法规则与产品测算规则共用同一版本方案：此处的 version / effectiveDate
// 对应后端 capacity.DefaultAssumptions() 的 RuleVersion / EffectiveDate（#66），
// 二者应保持同步，避免出现两套并存的版本说明。
export interface MethodRuleMeta {
  version: string;
  effectiveDate: string; // ISO 日期
  updatedAt: string; // ISO 日期
  applicableScope: string; // 适用城市 / 市场阶段
  sampleRequirement: string; // 样本要求
  source: string; // 来源 / 依据
  limitation: string; // 局限
}

export const methodRuleMeta: MethodRuleMeta = {
  version: "2026.07",
  effectiveDate: "2026-07-01",
  updatedAt: "2026-07-01",
  applicableScope: "适用于二手房挂牌/成交较活跃的城市与板块；一手主导或成交样本过少的板块需谨慎套用。",
  sampleRequirement: "需要该板块连续多周、样本量足够的挂牌与成交观察，样本不足时结论仅供参考。",
  source: "基于房脉对目标板块挂牌、成交、降价与带看数据的持续观察，非普适定律。",
  limitation: "方法给出的是方向性判断，不承诺固定涨跌幅度或具体砍价比例。",
};
