// Breed display metadata for the 养熟 UI. The `client_id` override path of the
// distill endpoint takes a BREED id (not a CLI id); these are the top-level
// dog ids from packs/default/breeds/dog-template.json.

export interface BreedMeta {
  id: string;
  label: string;
  // Tailwind text/bg tone used for the avatar dot + badges (SG dark theme).
  tone: string;
  dot: string;
}

export const BREED_META: Record<string, BreedMeta> = {
  bianmu: { id: 'bianmu', label: '边牧', tone: 'text-blue-300', dot: 'bg-blue-400' },
  jinmao: { id: 'jinmao', label: '金毛', tone: 'text-amber-300', dot: 'bg-amber-400' },
  xigou: { id: 'xigou', label: '灵缇', tone: 'text-purple-300', dot: 'bg-purple-400' },
  demu: { id: 'demu', label: '德牧', tone: 'text-rose-300', dot: 'bg-rose-400' },
  zangao: { id: 'zangao', label: '藏獒', tone: 'text-orange-300', dot: 'bg-orange-400' },
  zhonghuatianyuanquan: { id: 'zhonghuatianyuanquan', label: '中华田园犬', tone: 'text-emerald-300', dot: 'bg-emerald-400' },
  leader: { id: 'leader', label: '大当家', tone: 'text-slate-300', dot: 'bg-slate-400' },
};

export const BREED_OPTIONS: BreedMeta[] = [
  BREED_META.bianmu,
  BREED_META.jinmao,
  BREED_META.xigou,
  BREED_META.demu,
  BREED_META.zangao,
  BREED_META.zhonghuatianyuanquan,
  BREED_META.leader,
];

export function breedLabel(id: string): string {
  return BREED_META[id]?.label ?? id;
}

export function breedDot(id: string): string {
  return BREED_META[id]?.dot ?? 'bg-slate-500';
}
