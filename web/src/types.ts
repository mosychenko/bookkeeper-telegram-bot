export type Payment = {
  name: string;
  amount: number;
  due: string;
  paid: boolean;
  paidNote?: string;
};

export type FlexRow = {
  name: string;
  plan: number | null;
  fact: number;
  left: number | null;
  lastMonth: number;
  overspent: boolean;
};

export type Spent = { name: string; amount: number };

export type View = {
  title: string;
  asOf: string;
  periodLabel: string;
  days: number;
  question: string;
  status: "ok" | "tight" | "no";
  statusText: string;
  free: number;
  perDay: number;
  freeCaption: string;
  perDayCaption: string;
  riskText: string;
  mandatory: {
    heading: string;
    caption: string;
    rows: Payment[];
    total: number;
    covered: boolean;
    shortfall: number;
    totalText: string;
  };
  flexible: {
    heading: string;
    caption: string;
    prevMonth: string;
    rows: FlexRow[];
  };
  cover: {
    heading: string;
    lines: string[];
  };
  top: {
    heading: string;
    column: string;
    note: string;
    rows: Spent[];
  };
  warnings: string[];
  sourceNote: string;
};
