import type { BreedResponseStartEvent } from '../../types';
import { getBreedColor } from '../../lib/breed-colors';
import { useBreeds } from '../../hooks/useBreeds';

interface BreedResponseStartProps {
  event: BreedResponseStartEvent;
}

export function BreedResponseStart({ event }: BreedResponseStartProps) {
  const { dogs } = useBreeds();
  const dog = dogs.find((d) => d.id === event.breed);
  const color = dog?.color ?? getBreedColor(event.breed).primary;
  const name = dog?.name ?? event.breed;

  return (
    <div className="flex items-center gap-2 my-2 text-xs">
      <div className="w-6 h-6 rounded-lg flex items-center justify-center text-white shadow" style={{ backgroundColor: color }}>
        <i className="fa-solid fa-dog text-[10px]"></i>
      </div>
      <span className="font-bold text-slate-200">{name}</span>
      <span className="text-slate-500 font-mono text-[10px]">started · {event.timestamp}</span>
      <span className="flex-1 h-px bg-slate-800"></span>
    </div>
  );
}
