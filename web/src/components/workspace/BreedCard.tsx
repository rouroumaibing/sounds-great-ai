import type { BreedCardEvent } from '../../types';
import { getBreedColor } from '../../lib/breed-colors';
import { useBreeds } from '../../hooks/useBreeds';
import { Markdown } from '../common/Markdown';
import { ThinkingBlock } from './ThinkingBlock';
import { ToolLogBlock } from './ToolLogBlock';

interface BreedCardProps {
  event: BreedCardEvent;
}

export function BreedCard({ event }: BreedCardProps) {
  const { dogs } = useBreeds();
  const dog = dogs.find((d) => d.id === event.breedId);
  const color = dog?.color ?? getBreedColor(event.breedId).primary;
  const icon = dog?.icon ?? '';
  const name = dog?.name ?? event.breedId;

  return (
    <div className="flex gap-3 relative group">
      {/* Timeline connector line */}
      <div className="absolute left-4 top-10 bottom-0 w-0.5 bg-slate-800 -z-0"></div>

      {/* Breed Avatar */}
      <div
        className="w-8 h-8 rounded-xl flex items-center justify-center text-white shrink-0 z-10 shadow-lg text-xs"
        style={{ backgroundColor: color }}
      >
        <i className={icon}></i>
      </div>

      {/* Card Body */}
      <div className="flex-1 bg-slate-900/80 border border-slate-800 rounded-2xl p-4 space-y-3 shadow-md">
        {/* Card Header */}
        <div className="flex items-center justify-between border-b border-slate-800/80 pb-2">
          <div className="flex items-center space-x-2">
            <span className="text-xs font-bold text-slate-100">{name}</span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
              {event.role}
            </span>
            <span className="text-[10px] font-mono text-slate-500 border-l border-slate-800 pl-2">
              <i className="fa-solid fa-microchip text-[9px] mr-1"></i>{event.model}
            </span>
          </div>
          <span className="text-[10px] font-mono text-slate-500">{event.timestamp}</span>
        </div>

        {/* Thinking Process Collapsible */}
        {event.thinking && (
          <ThinkingBlock thinking={event.thinking} showThinking={event.showThinking} />
        )}

        {/* Main Content Text */}
        <div className="text-xs text-slate-300 leading-relaxed space-y-2">
          <Markdown>{event.content}</Markdown>
        </div>

        {/* Tool Executions Log Block */}
        {event.tools.map((tool, tIdx) => (
          <ToolLogBlock
            key={tIdx}
            event={{ type: 'tool_call', tool: tool.name, params: tool.args, result: tool.output, status: 'success' }}
          />
        ))}
      </div>
    </div>
  );
}
