import { describe, expect, it } from "vitest";

import { buildTemplateMarkdown, decisionTemplates } from "./template-catalog";

describe("decision template catalog", () => {
  it("defines six stable, versioned, sectioned templates", () => {
    expect(decisionTemplates).toHaveLength(6);
    expect(new Set(decisionTemplates.map((template) => template.id)).size).toBe(6);
    for (const template of decisionTemplates) {
      expect(template.id).toMatch(/^[a-z][a-z0-9-]+$/u);
      expect(template.version).toMatch(/^\d+\.\d+\.\d+$/u);
      expect(template.description.length).toBeGreaterThan(10);
      expect(template.sections.length).toBeGreaterThanOrEqual(5);
      expect(template.sections.every((section) => section.fields.length >= 4)).toBe(true);
    }
  });

  it("renders every catalog field into marked Markdown", () => {
    for (const template of decisionTemplates) {
      const markdown = buildTemplateMarkdown(template);
      expect(markdown).toContain(
        `<!-- propulse-template id="${template.id}" version="${template.version}" -->`,
      );
      expect(markdown).toContain(`模板 ID：\`${template.id}\``);
      expect(markdown).toContain(`模板版本：\`${template.version}\``);
      for (const section of template.sections) {
        expect(markdown).toContain(`## ${section.title}`);
        for (const field of section.fields) {
          expect(markdown).toContain(`- ${field}：`);
        }
      }
    }
  });

  it("keeps representative decision evidence in each template", () => {
    const markdownById = Object.fromEntries(
      decisionTemplates.map((template) => [template.id, buildTemplateMarkdown(template)]),
    );
    expect(markdownById["housing-budget"]).toContain("可接受极限月供（元）");
    expect(markdownById["neighborhood-watch"]).toContain("数据源 ID");
    expect(markdownById["weekly-monitoring"]).toContain("数据缺口说明");
    expect(markdownById["viewing-record"]).toContain("历史降价记录");
    expect(markdownById["negotiation-checklist"]).toContain("绝对最高出价");
    expect(markdownById["decision-review"]).toContain("是否忽略数据质量");
  });
});
