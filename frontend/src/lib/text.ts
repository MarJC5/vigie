// Lower case without accents, so "écoles" matches "ECOLES": the search, and tags compared as the
// Go side compares them.
export function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
}
