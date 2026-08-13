import type { BreedResponseCompleteEvent } from '../../types';
import { getBreedColor } from '../../lib/breed-colors';
import { useBreeds } from '../../hooks/useBreeds';

interface BreedResponseCompleteProps {
  event: BreedResponseCompleteEvent;
}

export function BreedResponseComplete({ event }: BreedResponseCompleteProps) {
  const { dogs } = useBreeds();
  const dog = dogs.find((d) => d.id === event.breed);
  const color = dog?.color ?? getBreedColor(event.breed).primary;
  const name = dog?.name ?? event.breed;
  const stepCount = Array.isArray(event.steps) ? event.steps.length : 0;

  return (
    <div className="my-2">
      <div className="flex items-center gap-2 text-xs">
        <div className="w-6 h-6 rounded-lg flex items-center justify-center text-white shadow" style={{ backgroundColor: color }}>
          <i className="fa-solid fa-check text-[10px]"></i>
        </div>
        <span className="font-bold text-slate-200">{name}</span>
        <span className="text-emerald-400 font-mono text-[10px]">completed · {stepCount} steps</span>
        <span className="flex-1 h-px bg-slate-800"></span>
      </div>
      {event.content ? (
        <div className="ml-8 mt-1 whitespace-pre-wrap rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 py-2 text-sm leading-relaxed text-slate-200">
          {event.content}
        </div>
      ) : null}
    </div>
  );
}
