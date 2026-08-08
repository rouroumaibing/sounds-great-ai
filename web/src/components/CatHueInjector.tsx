import { useEffect } from 'react';
import { useBreeds } from '../hooks/useBreeds';

function hexToHsl(hex: string): { h: number; s: number; l: number } {
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  const max = Math.max(r, g, b), min = Math.min(r, g, b);
  let h = 0, s = 0;
  const l = (max + min) / 2;
  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case r: h = (g - b) / d + (g < b ? 6 : 0); break;
      case g: h = (b - r) / d + 2; break;
      case b: h = (r - g) / d + 4; break;
    }
    h /= 6;
  }
  return { h: h * 360, s: s * 100, l: l * 100 };
}

export function CatHueInjector() {
  const { dogs } = useBreeds();
  useEffect(() => {
    const root = document.documentElement;
    dogs.forEach(dog => {
      const color = dog.color || '#9B7EBD';
      const { h, s, l } = hexToHsl(color);
      root.style.setProperty(`--${dog.id}-hue`, `${h}`);
      root.style.setProperty(`--${dog.id}-chroma`, `${s}%`);
      root.style.setProperty(`--color-${dog.id}-primary`, color);
      root.style.setProperty(`--color-${dog.id}-surface`, `hsl(${h}, ${s}%, ${l + 10}%)`);
      root.style.setProperty(`--color-${dog.id}-text`, `hsl(${h}, ${s}%, ${Math.max(l - 20, 10)}%)`);
    });
  }, [dogs]);
  return null;
}
