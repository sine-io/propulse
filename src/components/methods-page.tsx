import Link from "next/link";

const topics = [
  "挂牌变多但成交弱，说明什么？",
  "为什么不能只看挂牌均价？",
  "什么是真正的“买方窗口”？",
  "降价房源变多，一定会大跌吗？",
  "改善换房为什么要看新旧差？",
  "为什么月供安全线比总价重要？",
];

export function MethodsPage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <section className="mx-auto mb-8 max-w-2xl text-center">
        <h1 className="text-3xl font-bold text-slate-900">
          不是死记术语，而是学会判断
        </h1>
        <p className="mt-3 text-slate-500">
          每一个判断方法都对应一个真实的买房难题。掌握底层逻辑，你就不再需要听信中介的涨跌预测。
        </p>
      </section>

      <section className="flex flex-col gap-8 md:flex-row">
        <aside className="w-full md:w-1/3 lg:w-1/4">
          <div className="sticky top-6 overflow-hidden rounded-xl border border-slate-200 bg-white">
            <h2 className="border-b border-slate-100 bg-slate-50 p-4 font-semibold text-slate-800">
              问题场景目录
            </h2>
            <nav className="text-sm font-medium text-slate-600" aria-label="问题场景目录">
              {topics.map((topic, index) => (
                <a
                  key={topic}
                  href="#main-method"
                  className={`block cursor-pointer border-l-4 px-4 py-3 transition-colors ${
                    index === 0
                      ? "border-blue-600 bg-blue-50 text-blue-700"
                      : "border-transparent hover:border-slate-300 hover:bg-slate-50"
                  }`}
                >
                  {topic}
                </a>
              ))}
            </nav>
          </div>
        </aside>

        <div className="w-full space-y-6 md:w-2/3 lg:w-3/4">
          <article
            id="main-method"
            className="rounded-2xl border border-slate-200 bg-white p-8 shadow-sm"
          >
            <h2 className="mb-6 text-2xl font-bold text-slate-900">
              挂牌变多但成交弱，说明什么？
            </h2>

            <div className="space-y-8">
              <section>
                <h3 className="mb-2 flex items-center font-bold text-rose-600">
                  <span className="mr-2 flex h-6 w-6 items-center justify-center rounded-full bg-rose-100 text-sm">
                    ✕
                  </span>
                  常见误判
                </h3>
                <p className="rounded-lg border border-slate-100 bg-slate-50 p-3 italic text-slate-600">
                  “小区挂牌量越来越多了，供大于求，房价肯定马上要暴跌，我再等等。”
                </p>
              </section>

              <section>
                <h3 className="mb-2 flex items-center font-bold text-emerald-600">
                  <span className="mr-2 flex h-6 w-6 items-center justify-center rounded-full bg-emerald-100 text-sm">
                    ✓
                  </span>
                  正确看法
                </h3>
                <p className="leading-relaxed text-slate-700">
                  单纯看挂牌量增加是没有意义的，必须要结合
                  <strong className="text-slate-900">成交量</strong>
                  一起看。如果挂牌持续增加，但成交量没有同步放大（甚至萎缩），这意味着
                  <strong className="bg-blue-100 px-1 text-blue-800">
                    库存积压严重，房东的竞争加剧
                  </strong>
                  。这时候，部分急缺资金的房东为了成交，会率先打破价格僵局，你的议价空间（砍价余地）被打开了。
                </p>
              </section>

              <section>
                <h3 className="mb-3 border-b border-slate-200 pb-2 font-bold text-slate-800">
                  你需要盯住的关键指标
                </h3>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="rounded-lg border border-slate-100 bg-white p-3">
                    <p className="text-sm font-semibold text-slate-800">在售套数增幅</p>
                    <p className="mt-1 text-xs text-slate-500">连续四周增加是个强信号。</p>
                  </div>
                  <div className="rounded-lg border border-slate-100 bg-white p-3">
                    <p className="text-sm font-semibold text-slate-800">降价房源占比</p>
                    <p className="mt-1 text-xs text-slate-500">
                      超过 20% 的在售房源下调过报价。
                    </p>
                  </div>
                </div>
              </section>

              <section className="rounded-xl border border-blue-100 bg-blue-50 p-5">
                <h3 className="mb-2 font-bold text-blue-900">
                  你应该怎么用这个知识？
                </h3>
                <p className="text-sm leading-relaxed text-blue-800">
                  当在你的目标小区看到这个信号时：
                  <br />
                  1. <strong className="text-blue-900">开始看房：</strong>{" "}
                  因为房源多，你有充分的挑选余地。
                  <br />
                  2. <strong className="text-blue-900">不急下定：</strong>{" "}
                  不要怕被别人抢走，耐心挑瑕疵。
                  <br />
                  3. <strong className="text-blue-900">大胆砍价：</strong>{" "}
                  专挑挂牌时间长、有过降价记录的房源，按照成交底价再往下砍 3%-5%
                  试探底线。
                </p>
              </section>
            </div>
          </article>

          <section className="rounded-2xl bg-slate-900 p-6 text-center text-white">
            <h3 className="mb-2 font-bold">学以致用：本周复盘练习</h3>
            <p className="mb-4 text-sm text-slate-400">
              去你的目标小区详情页，记录本周的挂牌和成交变化，写下你的行动判断。
            </p>
            <Link
              href="/neighborhoods"
              className="inline-flex rounded-lg bg-white px-6 py-2 font-medium text-slate-900 transition-colors hover:bg-slate-100"
            >
              前往目标小区实践
            </Link>
          </section>
        </div>
      </section>
    </main>
  );
}
