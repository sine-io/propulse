CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE capacity_policy_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  city TEXT NOT NULL CHECK (char_length(city) BETWEEN 1 AND 128),
  version TEXT NOT NULL CHECK (char_length(version) BETWEEN 1 AND 128),
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 256),
  effective_from DATE NOT NULL,
  effective_to DATE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  rules JSONB NOT NULL,
  sources JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT capacity_policy_effective_range_valid
    CHECK (effective_to IS NULL OR effective_to > effective_from),
  CONSTRAINT capacity_policy_sources_nonempty
    CHECK (jsonb_typeof(sources) = 'array' AND jsonb_array_length(sources) > 0),
  UNIQUE (city, version),
  EXCLUDE USING gist (
    city WITH =,
    daterange(effective_from, COALESCE(effective_to, 'infinity'::date), '[)') WITH &&
  ) WHERE (enabled)
);

CREATE INDEX idx_capacity_policy_versions_city_effective
  ON capacity_policy_versions(city, effective_from DESC);

INSERT INTO capacity_policy_versions (
  id,
  city,
  version,
  name,
  effective_from,
  enabled,
  rules,
  sources
) VALUES (
  '8f297575-9418-4ea8-b901-c6467a639a01',
  '天津',
  'tianjin-2026.01.01',
  '天津住房交易与贷款预算政策（2026-01）',
  DATE '2026-01-01',
  TRUE,
  '{
    "DownPayment": {
      "CommercialFirst": 0.15,
      "CommercialSecond": 0.15,
      "ProvidentFirst": 0.20,
      "ProvidentSecond": 0.20,
      "CombinedFirst": 0.20,
      "CombinedSecond": 0.20
    },
    "Interest": {
      "CommercialFirst": 0.0305,
      "CommercialSecond": 0.0305,
      "ProvidentFirstUpToFiveYears": 0.021,
      "ProvidentFirstOverFiveYears": 0.026,
      "ProvidentSecondUpToFiveYears": 0.02525,
      "ProvidentSecondOverFiveYears": 0.03075
    },
    "Tax": {
      "DeedFirstUpToAreaRate": 0.01,
      "DeedFirstOverAreaRate": 0.015,
      "DeedSecondUpToAreaRate": 0.01,
      "DeedSecondOverAreaRate": 0.02,
      "DeedAreaThresholdSQM": 140,
      "VATRate": 0.03,
      "VATExemptHoldingYears": 2,
      "VATSurchargeRate": 0.06,
      "IncomeTaxGainRate": 0.20,
      "IncomeTaxAssessedRate": 0.01,
      "IncomeTaxExemptHoldingYears": 5
    }
  }'::jsonb,
  '[
    {
      "Code": "commercial_down_payment",
      "Title": "天津市进一步优化房地产政策：商业性个人住房贷款最低首付款比例不再区分首套、二套",
      "Issuer": "天津市住房和城乡建设委员会",
      "URL": "https://zfcxjs.tj.gov.cn/xxgk_70/zcjdx/202410/t20241016_6754643.html",
      "EffectiveDate": "2024-10-16"
    },
    {
      "Code": "commercial_rate",
      "Title": "天津住房商业贷款参考利率报道（实际利率以贷款机构为准）",
      "Issuer": "天津日报（天津政务网）",
      "URL": "https://www.tj.gov.cn/zmhd/hygqx/202505/t20250508_6926080.html",
      "EffectiveDate": "2025-05-08"
    },
    {
      "Code": "provident_down_payment",
      "Title": "关于调整个人住房公积金贷款有关政策的通知政策解读",
      "Issuer": "天津市住房公积金管理中心",
      "URL": "https://www.tj.gov.cn/zmhd/WDK/WDZT/ZCZT/202504/t20250425_6917983.html",
      "EffectiveDate": "2025-04-01"
    },
    {
      "Code": "provident_fund",
      "Title": "关于下调个人住房公积金贷款利率的天津执行说明",
      "Issuer": "中国人民银行、天津市住房公积金管理中心",
      "URL": "https://www.tj.gov.cn/zmhd/hygqx/202505/t20250508_6926080.html",
      "EffectiveDate": "2025-05-08"
    },
    {
      "Code": "deed_tax",
      "Title": "关于促进房地产市场平稳健康发展有关税收政策的公告",
      "Issuer": "财政部、税务总局、住房城乡建设部",
      "URL": "https://fgk.chinatax.gov.cn/zcfgk/c102416/c5235817/content.html",
      "EffectiveDate": "2024-12-01"
    },
    {
      "Code": "housing_vat",
      "Title": "关于个人销售住房增值税政策的公告",
      "Issuer": "财政部、税务总局",
      "URL": "https://fgk.chinatax.gov.cn/zcfgk/c102416/c5246356/content.html",
      "EffectiveDate": "2026-01-01"
    },
    {
      "Code": "tax_surcharges",
      "Title": "六税两费减征政策问答",
      "Issuer": "国家税务总局",
      "URL": "https://www.chinatax.gov.cn/chinatax/n810356/n3010387/c5237837/content.html",
      "EffectiveDate": "2023-01-01"
    },
    {
      "Code": "individual_income_tax",
      "Title": "关于个人住房转让所得征收个人所得税有关问题的通知",
      "Issuer": "国家税务总局",
      "URL": "https://fgk.chinatax.gov.cn/zcfgk/c100012/c5193804/content.html",
      "EffectiveDate": "2006-08-01"
    }
  ]'::jsonb
);
