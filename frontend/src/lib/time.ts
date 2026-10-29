// How long ago something happened, in the interface language ("3 minutes ago", "il y a 3 minutes").
const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
];

export function since(iso: string, lang: string, now: Date = new Date()): string {
  const seconds = Math.round((now.getTime() - new Date(iso).getTime()) / 1000);
  const format = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
  for (const [unit, size] of units) {
    if (seconds >= size) return format.format(-Math.floor(seconds / size), unit);
  }
  return format.format(0, "second");
}
