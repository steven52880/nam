import { describe, expect, it } from 'vitest';
import { statsWindowLabel } from './labels';
import {
  MAX_WINDOW_DAYS,
  MAX_WINDOW_HOURS,
  buildStatsWindow,
  isPresetWindow,
  parseCustomWindow,
  validateWindowInput,
} from './statsWindow';

describe('validateWindowInput', () => {
  it('合法的正整数通过', () => {
    expect(validateWindowInput('12', 'h')).toBeNull();
    expect(validateWindowInput(' 30 ', 'd')).toBeNull();
    expect(validateWindowInput(String(MAX_WINDOW_HOURS), 'h')).toBeNull();
    expect(validateWindowInput(String(MAX_WINDOW_DAYS), 'd')).toBeNull();
  });

  it('空、非整数、0、负数、超上限都不通过', () => {
    expect(validateWindowInput('', 'h')).not.toBeNull();
    expect(validateWindowInput('  ', 'd')).not.toBeNull();
    expect(validateWindowInput('1.5', 'h')).not.toBeNull();
    expect(validateWindowInput('-3', 'd')).not.toBeNull();
    expect(validateWindowInput('1e3', 'h')).not.toBeNull();
    expect(validateWindowInput('0', 'h')).not.toBeNull();
    expect(validateWindowInput('00', 'd')).not.toBeNull();
    expect(validateWindowInput(String(MAX_WINDOW_HOURS + 1), 'h')).not.toBeNull();
    expect(validateWindowInput(String(MAX_WINDOW_DAYS + 1), 'd')).not.toBeNull();
  });

  it('上限：4464 小时 = 186 天', () => {
    expect(MAX_WINDOW_HOURS).toBe(4464);
    expect(MAX_WINDOW_DAYS).toBe(186);
  });
});

describe('buildStatsWindow / parseCustomWindow', () => {
  it('拼出并能解析回来', () => {
    expect(buildStatsWindow(12, 'h')).toBe('12h');
    expect(buildStatsWindow(30, 'd')).toBe('30d');
    expect(parseCustomWindow('12h')).toEqual({ amount: 12, unit: 'h' });
    expect(parseCustomWindow('3d')).toEqual({ amount: 3, unit: 'd' });
  });

  it('不合法返回 null', () => {
    expect(buildStatsWindow(0, 'h')).toBeNull();
    expect(buildStatsWindow(1.5, 'd')).toBeNull();
    expect(buildStatsWindow(187, 'd')).toBeNull();
    expect(buildStatsWindow(4465, 'h')).toBeNull();
    expect(parseCustomWindow('latest')).toBeNull();
    expect(parseCustomWindow('0h')).toBeNull();
    expect(parseCustomWindow('05h')).toBeNull();
    expect(parseCustomWindow('12')).toBeNull();
    expect(parseCustomWindow('12m')).toBeNull();
    expect(parseCustomWindow('187d')).toBeNull();
  });
});

describe('statsWindowLabel / isPresetWindow', () => {
  it('预设与自定义的文字', () => {
    expect(statsWindowLabel('latest')).toBe('最新');
    expect(statsWindowLabel('3d')).toBe('3 天');
    expect(statsWindowLabel('7d')).toBe('7 天');
    expect(statsWindowLabel('12h')).toBe('12 小时');
    expect(statsWindowLabel('30d')).toBe('30 天');
  });

  it('认不出的值原样返回', () => {
    expect(statsWindowLabel('999d')).toBe('999d');
  });

  it('只有 latest / 3d / 7d 是预设', () => {
    expect(isPresetWindow('latest')).toBe(true);
    expect(isPresetWindow('3d')).toBe(true);
    expect(isPresetWindow('7d')).toBe(true);
    expect(isPresetWindow('72h')).toBe(false);
    expect(isPresetWindow('30d')).toBe(false);
  });
});
