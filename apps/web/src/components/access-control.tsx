"use client";

import { Eye, EyeOff, KeyRound, LockKeyhole, LogOut, X } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";

import {
  clearAccessToken,
  getAccessToken,
  setAccessToken,
  subscribeToAccessToken,
} from "@/lib/access-token";
import { ApiError, verifyAccessToken } from "@/lib/api-client";

interface AccessControlProps {
  reloadPage?: () => void;
}

export function AccessControl({
  reloadPage = () => window.location.reload(),
}: AccessControlProps = {}) {
  const [isOpen, setIsOpen] = useState(false);
  const [isUnlocked, setIsUnlocked] = useState(false);
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    const refresh = () => setIsUnlocked(Boolean(getAccessToken()));
    refresh();
    return subscribeToAccessToken(refresh);
  }, []);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const candidate = token.trim();
    if (!candidate) {
      setError("请输入访问令牌。");
      return;
    }

    setIsSubmitting(true);
    setError(undefined);
    try {
      await verifyAccessToken(candidate);
      setAccessToken(candidate);
      setToken("");
      setIsOpen(false);
      reloadPage();
    } catch (caught) {
      setError(
        caught instanceof ApiError && caught.status === 401
          ? "访问令牌无效。"
          : "暂时无法验证访问令牌。",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const lock = () => {
    clearAccessToken();
    setIsOpen(false);
    reloadPage();
  };

  return (
    <>
      <button
        type="button"
        onClick={() => {
          setError(undefined);
          setIsOpen(true);
        }}
        className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50"
      >
        {isUnlocked ? (
          <LockKeyhole aria-hidden="true" className="h-4 w-4 text-emerald-600" />
        ) : (
          <KeyRound aria-hidden="true" className="h-4 w-4" />
        )}
        <span className="hidden sm:inline">{isUnlocked ? "已解锁" : "解锁"}</span>
      </button>

      {isOpen ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="access-dialog-title"
          className="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/45 px-4"
        >
          <div className="w-full max-w-md rounded-lg border border-slate-200 bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4">
              <h2 id="access-dialog-title" className="text-base font-semibold text-slate-900">
                {isUnlocked ? "个人空间已解锁" : "解锁个人空间"}
              </h2>
              <button
                type="button"
                aria-label="关闭"
                onClick={() => setIsOpen(false)}
                className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-slate-900"
              >
                <X aria-hidden="true" className="h-4 w-4" />
              </button>
            </div>

            {isUnlocked ? (
              <div className="p-5">
                <p className="text-sm text-slate-600">当前浏览器会话可以访问个人测算、观察池和决策数据。</p>
                <button
                  type="button"
                  onClick={lock}
                  className="mt-5 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-rose-200 text-sm font-medium text-rose-700 hover:bg-rose-50"
                >
                  <LogOut aria-hidden="true" className="h-4 w-4" />
                  锁定个人空间
                </button>
              </div>
            ) : (
              <form onSubmit={submit} className="p-5">
                <label htmlFor="access-token" className="mb-2 block text-sm font-medium text-slate-700">
                  访问令牌
                </label>
                <div className="relative">
                  <input
                    id="access-token"
                    type={showToken ? "text" : "password"}
                    value={token}
                    onChange={(event) => setToken(event.target.value)}
                    autoComplete="current-password"
                    className="h-11 w-full rounded-md border border-slate-300 px-3 pr-11 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  />
                  <button
                    type="button"
                    aria-label={showToken ? "隐藏访问令牌" : "显示访问令牌"}
                    onClick={() => setShowToken((value) => !value)}
                    className="absolute right-1.5 top-1.5 inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100"
                  >
                    {showToken ? <EyeOff aria-hidden="true" className="h-4 w-4" /> : <Eye aria-hidden="true" className="h-4 w-4" />}
                  </button>
                </div>
                {error ? <p role="alert" className="mt-2 text-sm text-rose-600">{error}</p> : null}
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="mt-5 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-slate-900 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <KeyRound aria-hidden="true" className="h-4 w-4" />
                  {isSubmitting ? "正在验证" : "验证并解锁"}
                </button>
              </form>
            )}
          </div>
        </div>
      ) : null}
    </>
  );
}
