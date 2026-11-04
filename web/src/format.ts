export function rub(v: number): string {
  const n = Math.round(v);
  const neg = n < 0;
  const grouped = Math.abs(n)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  return (neg ? "-" : "") + grouped + " ₽";
}

export function planText(v: number | null): string {
  return v == null ? "не задан" : rub(v);
}

export function leftText(v: number | null): string {
  return v == null ? "—" : rub(v);
}
