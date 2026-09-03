// foldText lowercases and strips accents so search matches "tete" <-> "Tete" etc. Lifted out of
// BandDetail (T127) so the songs list and the concerts list share one definition, not two copies.
export function foldText(s: string): string {
  return s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
}
