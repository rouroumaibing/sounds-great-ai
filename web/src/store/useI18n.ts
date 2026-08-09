import { create } from 'zustand';
import { zhCN } from '../i18n/zh-CN';
import { en } from '../i18n/en';

export type Locale = 'zh-CN' | 'en';

type Dict = Record<string, string>;

const dictionaries: Record<Locale, Dict> = {
  'zh-CN': zhCN,
  en,
};

interface I18nState {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, fallback?: string) => string;
}

const savedLocale = (typeof localStorage !== 'undefined' && localStorage.getItem('sga-locale')) as Locale | null;
const initialLocale: Locale = savedLocale === 'en' || savedLocale === 'zh-CN' ? savedLocale : 'zh-CN';

export const useI18n = create<I18nState>((set, get) => ({
  locale: initialLocale,
  setLocale: (locale) => {
    if (typeof localStorage !== 'undefined') localStorage.setItem('sga-locale', locale);
    set({ locale });
  },
  t: (key, fallback) => {
    const dict = dictionaries[get().locale];
    return dict[key] ?? fallback ?? key;
  },
}));
