"use client";

import { ChevronDown, LoaderCircle, RefreshCw, X } from "lucide-react";
import { ReactNode, useEffect, useId, useRef, useState } from "react";

export type SearchComboboxState = "idle" | "loading" | "ready" | "failed";

interface SearchComboboxProps<T> {
  label: string;
  placeholder: string;
  query: string;
  onQueryChange: (query: string) => void;
  items: T[];
  selectedItem?: T;
  onSelect: (item?: T) => void;
  getOptionId: (item: T) => string;
  getOptionLabel: (item: T) => string;
  renderOption?: (item: T) => ReactNode;
  state: SearchComboboxState;
  disabled?: boolean;
  loadingMessage?: string;
  emptyMessage?: string;
  failureMessage?: string;
  onRetry?: () => void;
}

export function SearchCombobox<T>({
  disabled = false,
  emptyMessage = "没有匹配结果",
  failureMessage = "读取失败",
  getOptionId,
  getOptionLabel,
  items,
  label,
  loadingMessage = "正在读取",
  onQueryChange,
  onRetry,
  onSelect,
  placeholder,
  query,
  renderOption,
  selectedItem,
  state,
}: SearchComboboxProps<T>) {
  const generatedID = useId();
  const listboxID = `search-combobox-${generatedID.replaceAll(":", "")}`;
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const displayValue = selectedItem ? getOptionLabel(selectedItem) : query;

  useEffect(() => {
    setActiveIndex(-1);
  }, [items, state]);

  useEffect(() => {
    if (state === "failed") setOpen(true);
  }, [state]);

  useEffect(() => {
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    return () => document.removeEventListener("pointerdown", closeOnOutsidePointer);
  }, []);

  const choose = (item: T) => {
    onQueryChange("");
    onSelect(item);
    setOpen(false);
    setActiveIndex(-1);
  };

  const clear = () => {
    onSelect(undefined);
    onQueryChange("");
    setOpen(true);
    setActiveIndex(-1);
    inputRef.current?.focus();
  };

  return (
    <div ref={rootRef} className="relative min-w-0">
      <label className="block">
        <span className="mb-1 block text-xs font-medium text-slate-600">{label}</span>
        <span className="relative block">
          <input
            ref={inputRef}
            role="combobox"
            aria-autocomplete="list"
            aria-controls={listboxID}
            aria-expanded={open && !disabled}
            aria-activedescendant={state === "ready" && activeIndex >= 0 && items[activeIndex] ? `${listboxID}-${getOptionId(items[activeIndex])}` : undefined}
            aria-label={label}
            autoComplete="off"
            disabled={disabled}
            placeholder={placeholder}
            value={displayValue}
            onChange={(event) => {
              if (selectedItem) onSelect(undefined);
              onQueryChange(event.target.value);
              setOpen(true);
            }}
            onClick={() => setOpen(true)}
            onFocus={() => setOpen(true)}
            onKeyDown={(event) => {
              if (disabled) return;
              if (event.key === "Escape") {
                event.preventDefault();
                setOpen(false);
                setActiveIndex(-1);
                return;
              }
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setOpen(true);
                if (state === "ready" && items.length) setActiveIndex((current) => current < items.length - 1 ? current + 1 : 0);
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setOpen(true);
                if (state === "ready" && items.length) setActiveIndex((current) => current > 0 ? current - 1 : items.length - 1);
                return;
              }
              if (event.key === "Enter" && state === "ready" && open && activeIndex >= 0 && items[activeIndex]) {
                event.preventDefault();
                choose(items[activeIndex]);
              }
            }}
            className="h-10 w-full min-w-0 rounded-md border border-slate-300 bg-white pl-2 pr-14 text-xs font-medium text-slate-900 outline-none placeholder:font-normal placeholder:text-slate-400 focus:border-blue-500 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500 sm:pl-3 sm:pr-16 sm:text-sm"
          />
          <span className="pointer-events-none absolute inset-y-0 right-2 flex items-center text-slate-400">
            {state === "loading" ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <ChevronDown aria-hidden="true" className="h-4 w-4" />}
          </span>
          {displayValue && !disabled ? (
            <button
              type="button"
              aria-label={`清空${label}`}
              title={`清空${label}`}
              onClick={clear}
              className="absolute inset-y-0 right-8 inline-flex w-7 items-center justify-center text-slate-400 hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            >
              <X aria-hidden="true" className="h-4 w-4" />
            </button>
          ) : null}
        </span>
      </label>

      {open && !disabled ? (
        <div
          id={listboxID}
          role="listbox"
          aria-label={`${label}选项`}
          className="absolute left-0 right-0 z-30 mt-1 max-h-72 overflow-y-auto rounded-md border border-slate-200 bg-white p-1 shadow-lg"
        >
          {state === "loading" ? (
            <div role="option" aria-selected="false" aria-disabled="true" className="flex min-h-10 items-center gap-2 px-3 py-2 text-sm text-slate-500">
              <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />
              <span role="status">{loadingMessage}</span>
            </div>
          ) : null}
          {state === "failed" ? (
            <div role="option" aria-selected="false" aria-disabled="true" className="flex min-h-10 items-center justify-between gap-3 px-3 py-2 text-sm text-rose-700">
              <span role="alert">{failureMessage}</span>
              {onRetry ? (
                <button type="button" onClick={onRetry} className="inline-flex flex-none items-center gap-1 font-medium hover:underline">
                  <RefreshCw aria-hidden="true" className="h-4 w-4" />重试
                </button>
              ) : null}
            </div>
          ) : null}
          {state === "ready" && items.length === 0 ? (
            <div role="option" aria-selected="false" aria-disabled="true" className="min-h-10 px-3 py-2 text-sm text-slate-500">{emptyMessage}</div>
          ) : null}
          {state === "ready" ? items.map((item, index) => {
            const optionID = getOptionId(item);
            const selected = selectedItem ? getOptionId(selectedItem) === optionID : false;
            return (
              <button
                key={optionID}
                id={`${listboxID}-${optionID}`}
                type="button"
                role="option"
                aria-selected={selected}
                onMouseDown={(event) => event.preventDefault()}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => choose(item)}
                className="block min-h-10 w-full rounded px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50 aria-selected:bg-blue-50 aria-selected:text-blue-950 data-[active=true]:bg-slate-100"
                data-active={activeIndex === index}
              >
                {renderOption ? renderOption(item) : getOptionLabel(item)}
              </button>
            );
          }) : null}
        </div>
      ) : null}
    </div>
  );
}
