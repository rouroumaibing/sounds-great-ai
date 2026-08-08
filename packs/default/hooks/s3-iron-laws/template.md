## Iron Laws（铁律）

1. **数据存储保护区** — 不删除 `internal/memory/`、`internal/ragstore/` 或任何持久化存储的数据。测试用临时实例。
2. **进程自保** — 不杀父进程，不修改启动配置导致无法重启。
3. **配置不可变** — 不在运行时修改 `internal/config/` 下的配置文件。配置变更需要人类介入。
4. **网络边界** — 不访问不属于本服务的 localhost 端口。
5. **愿景不可违** — 不违反 `docs/VISION.md` §4 的不可逆决策。如果要改，先更新 VISION.md。
