import { fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import { AppHeader } from "./app-header";
import { ActionWindowPage } from "./action-window-page";
import { CalculatorPanel } from "./calculator-panel";
import { HomePage } from "./home-page";
import { MethodsPage } from "./methods-page";
import { NeighborhoodsPage } from "./neighborhoods-page";
import { TemplatesPage } from "./templates-page";
import { WatchlistPage } from "./watchlist-page";

describe("AppHeader", () => {
  it("exposes all MVP navigation entries", () => {
    render(createElement(AppHeader));

    expect(screen.getByRole("link", { name: /房脉 proppulse/ })).toHaveAttribute(
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
});

describe("CalculatorPanel", () => {
  it("updates the diagnosis when target price becomes unsafe", () => {
    render(createElement(CalculatorPanel));

    expect(screen.getByText("月供压力：偏高")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("目标总价（万）"), {
      target: { value: "720" },
    });

    expect(screen.getByText("月供压力：危险")).toBeInTheDocument();
    expect(screen.getByText("暂缓改善")).toBeInTheDocument();
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
  });
});

describe("ActionWindowPage", () => {
  it("matches the reference action window recommendation", () => {
    render(createElement(ActionWindowPage));

    expect(screen.getByText("当前核心策略")).toBeInTheDocument();
    expect(screen.getByText("积极看房，大胆砍价")).toBeInTheDocument();
    expect(screen.getByText("策略执行信心")).toBeInTheDocument();
    expect(screen.getByText("风险警示")).toBeInTheDocument();
  });
});

describe("MethodsPage", () => {
  it("matches the reference methodology article structure", () => {
    render(createElement(MethodsPage));

    expect(screen.getByText("问题场景目录")).toBeInTheDocument();
    expect(screen.getByText("常见误判")).toBeInTheDocument();
    expect(screen.getByText("你需要盯住的关键指标")).toBeInTheDocument();
    expect(screen.getByText("前往目标小区实践")).toBeInTheDocument();
  });
});

describe("WatchlistPage", () => {
  it("matches the reference observation pool summary", () => {
    render(createElement(WatchlistPage));

    expect(screen.getByText("导出本周报表")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("小区动态 (本周变化)")).toBeInTheDocument();
    expect(screen.getByText("保存复盘记录")).toBeInTheDocument();
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
});
