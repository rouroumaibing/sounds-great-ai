import { describe, expect, it } from 'vitest';
import { useI18n } from './useI18n';
import { en } from '../i18n/en';
import { zhCN } from '../i18n/zh-CN';

describe('useI18n', () => {
  it('returns the zh-CN value by default', () => {
    useI18n.getState().setLocale('zh-CN');
    expect(useI18n.getState().t('people.title')).toBe('人物与关系记忆');
  });

  it('switches dictionaries on locale change and persists the choice', () => {
    useI18n.getState().setLocale('en');
    expect(useI18n.getState().t('people.title')).toBe('People & Relationship Memory');
    expect(localStorage.getItem('sga-locale')).toBe('en');
    useI18n.getState().setLocale('zh-CN');
  });

  it('falls back to the provided fallback, then the key', () => {
    useI18n.getState().setLocale('en');
    const t = useI18n.getState().t;
    expect(t('missing.key', 'fallback text')).toBe('fallback text');
    expect(t('missing.key')).toBe('missing.key');
  });

  it('en and zh-CN dictionaries keep exact key parity', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(zhCN).sort());
  });
});
