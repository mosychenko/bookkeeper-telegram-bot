import { useEffect, useState } from "react";
import type { View } from "./types";
import { leftText, planText, rub } from "./format";
import { bindTelegramTheme, initDataHeader } from "./theme";

export default function App() {
  const [data, setData] = useState<View | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    bindTelegramTheme();
    const header = initDataHeader();
    if (!header) {
      setError("Откройте мини-приложение из чата с ботом. Из обычного браузера сессию Telegram не проверить.");
      return;
    }
    const ctrl = new AbortController();
    fetch("/api/budget", {
      headers: { Authorization: `tma ${header}` },
      signal: ctrl.signal,
    })
      .then(async (res) => {
        const body = (await res.json().catch(() => null)) as { error?: string } | View | null;
        if (!res.ok) {
          throw new Error(body && "error" in body && body.error ? body.error : "Не удалось прочитать бюджет.");
        }
        setData(body as View);
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(err instanceof Error ? err.message : "Не удалось прочитать бюджет.");
      });
    return () => ctrl.abort();
  }, []);

  if (error) {
    return (
      <main className="state">
        <p>{error}</p>
      </main>
    );
  }
  if (!data) {
    return (
      <main className="state">
        <p>Считаю…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>{data.title}</h1>
      {data.warnings.length > 0 && (
        <div className="warn-list">
          {data.warnings.map((w) => (
            <p key={w}>{w}</p>
          ))}
        </div>
      )}

      <section className="card" aria-label="Итог">
        <p className="meta">
          Период {data.periodLabel} · данные по {data.asOf}
        </p>
        <p className="question">{data.question}</p>
        <div className={`status status-${data.status}`}>{data.statusText}</div>
        <div className="metrics">
          <div className="metric">
            <strong>{rub(data.free)}</strong>
            <span>{data.freeCaption}</span>
          </div>
          <div className="metric">
            <strong className={`status-${data.status}`}>{rub(data.perDay)}</strong>
            <span>{data.perDayCaption}</span>
          </div>
        </div>
        {data.riskText && <p className="risk">{data.riskText}</p>}
      </section>

      <section className="card">
        <h2>{data.mandatory.heading}</h2>
        <p className="caption">{data.mandatory.caption}</p>
        <div className="table-wrap">
          <table className="pays">
            <colgroup>
              <col className="c-name" />
              <col className="c-sum" />
              <col className="c-date" />
              <col className="c-state" />
            </colgroup>
            <thead>
              <tr>
                <th>Платёж</th>
                <th className="num">Сумма</th>
                <th>Дата</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              {data.mandatory.rows.map((row) => (
                <tr key={row.name}>
                  <td className="name">{row.name}</td>
                  <td className="num">{rub(row.amount)}</td>
                  <td className="due">{row.due}</td>
                  <td>
                    {row.paid ? <span className="paid">✓ оплачено</span> : <span className="wait">ожидает</span>}
                    {row.paid && row.paidNote ? <span className="paid-note">{row.paidNote}</span> : null}
                  </td>
                </tr>
              ))}
              <tr className="total">
                <td>Итого</td>
                <td className="num">{rub(data.mandatory.total)}</td>
                <td />
                <td className={data.mandatory.covered ? "ok" : "bad"}>{data.mandatory.totalText}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <h2>{data.flexible.heading}</h2>
        <p className="caption">{data.flexible.caption}</p>
        <div className="table-wrap">
          <table className="flex">
            <colgroup>
              <col className="c-name" />
              <col className="c-plan" />
              <col className="c-fact" />
              <col className="c-left" />
              <col className="c-prev" />
            </colgroup>
            <thead>
              <tr>
                <th>Статья</th>
                <th className="num">План</th>
                <th className="num">Факт</th>
                <th className="num">Осталось</th>
                <th className="num">{data.flexible.prevMonth}</th>
              </tr>
            </thead>
            <tbody>
              {data.flexible.rows.map((row) => (
                <tr key={row.name} className={row.overspent ? "over" : undefined}>
                  <td className="name">{row.name}</td>
                  <td className="num">{planText(row.plan)}</td>
                  <td className="num">{rub(row.fact)}</td>
                  <td className={row.left != null && row.left < 0 ? "num neg" : "num"}>{leftText(row.left)}</td>
                  <td className="num">{rub(row.lastMonth)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card cover">
        <h2>{data.cover.heading}</h2>
        {data.cover.lines.map((line) => (
          <p key={line}>{line}</p>
        ))}
      </section>

      <section className="card">
        <div className="section-head">
          <h2>{data.top.heading}</h2>
          <button type="button" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
            {open ? "Свернуть" : "Показать"}
          </button>
        </div>
        {open && (
          <>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>{data.top.column}</th>
                    <th className="num">Сумма</th>
                  </tr>
                </thead>
                <tbody>
                  {data.top.rows.length === 0 ? (
                    <tr>
                      <td colSpan={2}>Нет трат.</td>
                    </tr>
                  ) : (
                    data.top.rows.map((row) => (
                      <tr key={row.name}>
                        <td className="name">{row.name}</td>
                        <td className="num">{rub(row.amount)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
            {data.top.note && <p className="note">{data.top.note}</p>}
          </>
        )}
      </section>

      <p className="source">{data.sourceNote}</p>
    </main>
  );
}
