import { fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import { AppHeader } from "./app-header";
import { CalculatorPanel } from "./calculator-panel";
import { HomePage } from "./home-page";
import { TemplatesPage } from "./templates-page";

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
      "工具模板",
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
