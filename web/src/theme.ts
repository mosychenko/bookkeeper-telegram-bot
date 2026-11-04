const FALLBACK = {
  bg: "#efeff4",
  text: "#1c1c1e",
  hint: "#6d6d72",
  card: "#ffffff",
  link: "#2481cc",
  header: "#ffffff",
};

function paint(tp: TelegramThemeParams, dark: boolean) {
  const bg = tp.secondary_bg_color || (dark ? "#0f0f10" : FALLBACK.bg);
  const card = tp.section_bg_color || tp.bg_color || (dark ? "#1c1c1e" : FALLBACK.card);
  const text = tp.text_color || (dark ? "#f2f2f7" : FALLBACK.text);
  const hint = tp.subtitle_text_color || tp.hint_color || (dark ? "#98989f" : FALLBACK.hint);
  const link = tp.link_color || tp.button_color || FALLBACK.link;
  const header = tp.header_bg_color || tp.bg_color || card;
  const root = document.documentElement;
  root.style.setProperty("--bg", bg);
  root.style.setProperty("--card", card);
  root.style.setProperty("--text", text);
  root.style.setProperty("--hint", hint);
  root.style.setProperty("--link", link);
  root.style.colorScheme = dark ? "dark" : "light";
  const wa = window.Telegram?.WebApp;
  wa?.setHeaderColor?.(header);
  wa?.setBackgroundColor?.(bg);
}

export function bindTelegramTheme() {
  const apply = () => {
    const wa = window.Telegram?.WebApp;
    const tp = wa?.themeParams ?? {};
    const dark = wa?.colorScheme === "dark";
    paint(tp, dark);
  };
  apply();
  const wa = window.Telegram?.WebApp;
  wa?.ready();
  wa?.expand();
  wa?.onEvent?.("themeChanged", apply);
}

export function initDataHeader(): string | null {
  const fromTelegram = window.Telegram?.WebApp?.initData ?? "";
  if (fromTelegram) return fromTelegram;
  if (import.meta.env.VITE_DEV_AUTH === "1") return "dev";
  return null;
}
