import clsx from 'clsx';

interface FilterChip {
  id: string;
  label: string;
  activeClass: string;
}

interface FilterChipsProps {
  chips: FilterChip[];
  activeFilter: string;
  onFilterChange: (filter: string) => void;
}

export function FilterChips({ chips, activeFilter, onFilterChange }: FilterChipsProps) {
  return (
    <div className="flex items-center space-x-2 text-xs border-b border-slate-800/60 pb-3">
      {chips.map((chip) => (
        <button
          key={chip.id}
          onClick={() => onFilterChange(chip.id)}
          className={clsx(
            'px-3 py-1 rounded-lg border transition font-medium',
            activeFilter === chip.id
              ? chip.activeClass
              : 'border-slate-800 text-slate-400 hover:bg-slate-900'
          )}
        >
          {chip.label}
        </button>
      ))}
    </div>
  );
}
