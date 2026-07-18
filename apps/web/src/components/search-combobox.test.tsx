import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { SearchCombobox, type SearchComboboxState } from "./search-combobox";

const items = [
  { id: "one", name: "海河花园" },
  { id: "two", name: "梅江家园" },
];

function Harness({ state = "ready" }: { state?: SearchComboboxState }) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<(typeof items)[number]>();
  return (
    <SearchCombobox
      label="测试小区"
      placeholder="搜索"
      query={query}
      onQueryChange={setQuery}
      items={items}
      selectedItem={selected}
      onSelect={setSelected}
      getOptionId={(item) => item.id}
      getOptionLabel={(item) => item.name}
      state={state}
    />
  );
}

describe("SearchCombobox", () => {
  it("exposes combobox/listbox semantics and supports keyboard selection", () => {
    render(<Harness />);
    const input = screen.getByRole("combobox", { name: "测试小区" });
    fireEvent.focus(input);

    expect(input).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("listbox", { name: "测试小区选项" })).toBeInTheDocument();
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(input.getAttribute("aria-activedescendant")).toContain("two");
    fireEvent.keyDown(input, { key: "Enter" });

    expect(input).toHaveValue("梅江家园");
    expect(input).toHaveAttribute("aria-expanded", "false");
  });

  it("closes with Escape and clears a selected value", () => {
    render(<Harness />);
    const input = screen.getByRole("combobox", { name: "测试小区" });
    fireEvent.focus(input);
    fireEvent.click(screen.getByRole("option", { name: "海河花园" }));
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(input).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(screen.getByRole("button", { name: "清空测试小区" }));
    expect(input).toHaveValue("");
  });

  it("renders loading, empty, and retryable failure states", async () => {
    const onRetry = vi.fn();
    const { rerender } = render(
      <SearchCombobox
        label="状态选择"
        placeholder="搜索"
        query=""
        onQueryChange={() => undefined}
        items={[]}
        onSelect={() => undefined}
        getOptionId={(item: { id: string }) => item.id}
        getOptionLabel={(item) => item.id}
        state="loading"
        loadingMessage="加载选项"
      />,
    );
    fireEvent.focus(screen.getByRole("combobox", { name: "状态选择" }));
    expect(screen.getByRole("status")).toHaveTextContent("加载选项");

    rerender(
      <SearchCombobox
        label="状态选择"
        placeholder="搜索"
        query=""
        onQueryChange={() => undefined}
        items={[]}
        onSelect={() => undefined}
        getOptionId={(item: { id: string }) => item.id}
        getOptionLabel={(item) => item.id}
        state="ready"
        emptyMessage="没有结果"
      />,
    );
    expect(screen.getByText("没有结果")).toBeInTheDocument();

    rerender(
      <SearchCombobox
        label="状态选择"
        placeholder="搜索"
        query=""
        onQueryChange={() => undefined}
        items={[]}
        onSelect={() => undefined}
        getOptionId={(item: { id: string }) => item.id}
        getOptionLabel={(item) => item.id}
        state="failed"
        failureMessage="读取失败"
        onRetry={onRetry}
      />,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("读取失败");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
