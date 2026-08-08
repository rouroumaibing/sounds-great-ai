import clsx from 'clsx';
import { useBreeds } from '../../hooks/useBreeds';

export function DogPackGrid() {
  const { dogs, loading, error } = useBreeds();

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="p-3 border-b border-slate-800/80 flex items-center justify-between">
        <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
          <i className="fa-solid fa-dog text-indigo-400"></i>
          Dog Pack Roster ({dogs.length})
        </span>
        <span className="text-[10px] bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 px-1.5 py-0.5 rounded font-mono">CLI Ready</span>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
        {loading && <div className="text-center text-slate-500 text-xs py-4">加载中...</div>}
        {error && <div className="text-center text-rose-400 text-xs py-4">加载失败</div>}
        {!loading && !error && dogs.map((dog) => (
          <div
            key={dog.id}
            className="p-2.5 rounded-xl border transition cursor-pointer flex flex-col gap-1.5 bg-slate-950/40 border-slate-800/60 hover:bg-slate-900/60"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <div className="w-7 h-7 rounded-lg flex items-center justify-center text-xs font-bold text-white shadow" style={{ backgroundColor: dog.color }}>
                  <i className={dog.icon}></i>
                </div>
                <div>
                  <div className="flex items-center space-x-1.5">
                    <span className="text-xs font-bold text-slate-200">{dog.name}</span>
                  </div>
                  <div className="text-[10px] font-mono text-slate-400">{dog.role}</div>
                </div>
              </div>
              <span className={clsx('text-[9px] font-mono px-1.5 py-0.5 rounded border', dog.statusBadgeClass)}>
                {dog.statusText}
              </span>
            </div>

            <div className="flex items-center justify-between text-[10px] bg-slate-950/80 px-2 py-1 rounded-md border border-slate-800/80 font-mono text-slate-400">
              <span className="flex items-center gap-1">
                <i className="fa-solid fa-terminal text-[9px] text-slate-500"></i>
                {dog.adapter}
              </span>
              <span className="text-slate-500">{dog.latency}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
